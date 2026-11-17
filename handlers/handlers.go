package handlers

import (
	"crypto/subtle"
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"secret_santa/config"
	"secret_santa/logic"
	"secret_santa/models"
	"secret_santa/storage"
)

const (
	cookieParticipantSession = "intercambio_session"
	cookieAdminSession       = "intercambio_admin_session"
	sessionDurationSec       = 86400 * 365 // 1 year
)

type Server struct {
	cfg       *config.Config
	store     *storage.Store
	templates map[string]*template.Template
	staticFS  http.FileSystem
}

// NewServer initializes the server with templates and handlers
func NewServer(cfg *config.Config, store *storage.Store, tmplFS embed.FS, staticFS http.FileSystem) (*Server, error) {
	pages := []string{"index.html", "join.html", "reveal.html", "admin.html"}
	templates := make(map[string]*template.Template)

	for _, page := range pages {
		t, err := template.ParseFS(tmplFS, "templates/layout.html", "templates/"+page)
		if err != nil {
			return nil, fmt.Errorf("error al analizar plantilla %s: %w", page, err)
		}
		templates[page] = t
	}

	return &Server{
		cfg:       cfg,
		store:     store,
		templates: templates,
		staticFS:  staticFS,
	}, nil
}

// RegisterRoutes sets up all HTTP routes
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	// Static assets
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(s.staticFS)))

	// Flujos de participantes
	mux.HandleFunc("GET /{$}", s.handleHome)
	mux.HandleFunc("GET /code-redirect", s.handleCodeRedirect)
	mux.HandleFunc("GET /join/{code}", s.handleJoinPage)
	mux.HandleFunc("POST /join/{code}", s.handleJoinSubmit)
	mux.HandleFunc("GET /reveal", s.handleRevealSession)
	mux.HandleFunc("GET /reveal/{code}", s.handleRevealCode)
	mux.HandleFunc("GET /logout", s.handleParticipantLogout)

	// Panel de Control del Organizador
	mux.HandleFunc("GET /admin", s.handleAdminDashboard)
	mux.HandleFunc("GET /admin/logout", s.handleAdminLogout)
	mux.HandleFunc("POST /admin/generate-codes", s.requireAdmin(s.handleAdminGenerateCodes))
	mux.HandleFunc("POST /admin/roll", s.requireAdmin(s.handleAdminRoll))
	mux.HandleFunc("POST /admin/reset-draw", s.requireAdmin(s.handleAdminResetDraw))
	mux.HandleFunc("POST /admin/delete-participant", s.requireAdmin(s.handleAdminDeleteParticipant))
	mux.HandleFunc("POST /admin/update-event", s.requireAdmin(s.handleAdminUpdateEvent))
}

// render renders a full page using layout.html and the specific sub-template
func (s *Server) render(w http.ResponseWriter, tmplName string, data any) {
	tmpl, ok := s.templates[tmplName]
	if !ok {
		log.Printf("Plantilla no encontrada: %s", tmplName)
		http.Error(w, "Error interno del servidor", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := tmpl.ExecuteTemplate(w, "layout", data)
	if err != nil {
		log.Printf("Error al renderizar plantilla (%s): %v", tmplName, err)
		http.Error(w, "Error interno del servidor", http.StatusInternalServerError)
	}
}

// Participant Session Helpers
func (s *Server) getParticipantFromSession(r *http.Request) (models.Participant, bool) {
	cookie, err := r.Cookie(cookieParticipantSession)
	if err != nil || cookie.Value == "" {
		return models.Participant{}, false
	}
	return s.store.GetParticipantBySession(cookie.Value)
}

func (s *Server) setParticipantSession(w http.ResponseWriter, p models.Participant) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieParticipantSession,
		Value:    p.SessionToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   sessionDurationSec,
	})
}

// handleHome renders welcome/code input page, or redirects to /reveal if already logged in!
func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.getParticipantFromSession(r); ok {
		http.Redirect(w, r, "/reveal", http.StatusSeeOther)
		return
	}

	event := s.store.GetEvent()
	s.render(w, "index.html", map[string]any{
		"Title":            "Bienvenido",
		"EventTitle":       event.Title,
		"EventDescription": event.Description,
		"Error":            r.URL.Query().Get("error"),
		"Success":          r.URL.Query().Get("success"),
	})
}

// handleCodeRedirect redirects user-entered code from home page to /join/{code}
func (s *Server) handleCodeRedirect(w http.ResponseWriter, r *http.Request) {
	code := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("code")))
	if code == "" {
		http.Redirect(w, r, "/?error=Por+favor+ingresa+un+código+de+invitación", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/join/"+url.PathEscape(code), http.StatusSeeOther)
}

// handleJoinPage shows the registration form or starts session if already registered
func (s *Server) handleJoinPage(w http.ResponseWriter, r *http.Request) {
	code := strings.ToUpper(strings.TrimSpace(r.PathValue("code")))
	event := s.store.GetEvent()

	inv, exists := s.store.GetInviteCode(code)
	if !exists {
		s.render(w, "index.html", map[string]any{
			"Title":            "Código Inválido",
			"EventTitle":       event.Title,
			"EventDescription": event.Description,
			"Error":            fmt.Sprintf("El código de invitación '%s' no es válido. Por favor revisa tu enlace.", code),
		})
		return
	}

	// Si el código ya fue utilizado, restaura la sesión y muestra el intercambio
	if inv.Used {
		if p, ok := s.store.GetParticipantByCode(code); ok {
			s.setParticipantSession(w, p)
		}
		http.Redirect(w, r, "/reveal/"+url.PathEscape(code), http.StatusSeeOther)
		return
	}

	// Si el sorteo ya se realizó, el registro está cerrado
	if event.Status == models.StatusDrawn {
		s.render(w, "index.html", map[string]any{
			"Title":            "Registro Cerrado",
			"EventTitle":       event.Title,
			"EventDescription": event.Description,
			"Error":            "¡El sorteo del intercambio ya se realizó! El registro se encuentra cerrado.",
		})
		return
	}

	var currentP *models.Participant
	if p, ok := s.getParticipantFromSession(r); ok {
		currentP = &p
	}

	s.render(w, "join.html", map[string]any{
		"Title":              "Registro",
		"EventTitle":         event.Title,
		"EventDescription":   event.Description,
		"Code":               code,
		"Budget":             event.Budget,
		"CurrentParticipant": currentP,
		"Error":              r.URL.Query().Get("error"),
		"FormName":           r.URL.Query().Get("name"),
		"FormWishlist":       r.URL.Query().Get("wishlist"),
	})
}

// handleJoinSubmit processes the registration form submission without email/password!
func (s *Server) handleJoinSubmit(w http.ResponseWriter, r *http.Request) {
	code := strings.ToUpper(strings.TrimSpace(r.PathValue("code")))
	name := strings.TrimSpace(r.FormValue("name"))
	wishlist := strings.TrimSpace(r.FormValue("wishlist"))

	if name == "" {
		redirectWithError(w, r, fmt.Sprintf("/join/%s", code), "El nombre es obligatorio", name, wishlist)
		return
	}

	participant, err := s.store.RegisterParticipant(code, name, wishlist)
	if err != nil {
		redirectWithError(w, r, fmt.Sprintf("/join/%s", code), err.Error(), name, wishlist)
		return
	}

	log.Printf("[REGISTRADO] Participante %s registrado con código %s", participant.Name, code)

	// Inicia sesión automáticamente con la cookie
	s.setParticipantSession(w, *participant)

	http.Redirect(w, r, "/reveal?success=¡Bienvenido+al+intercambio!+Tu+sesión+ha+sido+iniciada.", http.StatusSeeOther)
}

func redirectWithError(w http.ResponseWriter, r *http.Request, targetURL, errMsg, name, wishlist string) {
	q := url.Values{}
	q.Set("error", errMsg)
	if name != "" {
		q.Set("name", name)
	}
	if wishlist != "" {
		q.Set("wishlist", wishlist)
	}
	http.Redirect(w, r, targetURL+"?"+q.Encode(), http.StatusSeeOther)
}

// handleRevealSession displays the participant's status based on their active session cookie
func (s *Server) handleRevealSession(w http.ResponseWriter, r *http.Request) {
	participant, ok := s.getParticipantFromSession(r)
	if !ok {
		http.Redirect(w, r, "/?error=Por+favor+ingresa+tu+código+de+invitación+para+consultar+tu+intercambio.", http.StatusSeeOther)
		return
	}

	s.renderParticipantReveal(w, r, participant)
}

// handleRevealCode displays participant status via code, and refreshes their session cookie
func (s *Server) handleRevealCode(w http.ResponseWriter, r *http.Request) {
	code := strings.ToUpper(strings.TrimSpace(r.PathValue("code")))

	participant, exists := s.store.GetParticipantByCode(code)
	if !exists {
		inv, invExists := s.store.GetInviteCode(code)
		if invExists && !inv.Used {
			http.Redirect(w, r, "/join/"+url.PathEscape(code), http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/?error=No+se+encontró+ningún+registro+para+este+código+de+invitación.", http.StatusSeeOther)
		return
	}

	// Mantener sesión abierta en este navegador
	s.setParticipantSession(w, participant)

	s.renderParticipantReveal(w, r, participant)
}

func (s *Server) renderParticipantReveal(w http.ResponseWriter, r *http.Request, participant models.Participant) {
	event := s.store.GetEvent()

	var receiver models.Participant
	if event.Status == models.StatusDrawn && participant.ReceiverID != "" {
		rec, found := s.store.GetParticipant(participant.ReceiverID)
		if found {
			receiver = rec
		}
	}

	s.render(w, "reveal.html", map[string]any{
		"Title":              fmt.Sprintf("Intercambio - %s", participant.Name),
		"EventTitle":         event.Title,
		"EventDescription":   event.Description,
		"Status":             event.Status,
		"Participant":        participant,
		"CurrentParticipant": &participant,
		"Receiver":           receiver,
		"Budget":             event.Budget,
		"Success":            r.URL.Query().Get("success"),
	})
}

// handleParticipantLogout logs out the participant
func (s *Server) handleParticipantLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieParticipantSession,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	http.Redirect(w, r, "/?success=Has+cerrado+sesión+correctamente.", http.StatusSeeOther)
}

// Admin Authentication Helpers
func (s *Server) isAdminAuthenticated(r *http.Request) bool {
	event := s.store.GetEvent()
	if event.AdminKey == "" {
		return false
	}

	if key := r.URL.Query().Get("key"); key != "" {
		if subtle.ConstantTimeCompare([]byte(key), []byte(event.AdminKey)) == 1 {
			return true
		}
	}

	cookie, err := r.Cookie(cookieAdminSession)
	if err == nil && cookie.Value != "" {
		if subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(event.AdminKey)) == 1 {
			return true
		}
	}

	return false
}

func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.isAdminAuthenticated(r) {
			http.Redirect(w, r, "/admin?error=Se+requiere+acceso+de+organizador.", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

// handleAdminDashboard renders organizer control panel
func (s *Server) handleAdminDashboard(w http.ResponseWriter, r *http.Request) {
	event := s.store.GetEvent()
	isAuth := s.isAdminAuthenticated(r)

	if key := r.URL.Query().Get("key"); key != "" && isAuth {
		http.SetCookie(w, &http.Cookie{
			Name:     cookieAdminSession,
			Value:    event.AdminKey,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   sessionDurationSec,
		})
	}

	if !isAuth {
		s.render(w, "admin.html", map[string]any{
			"Title":            "Acceso del Organizador",
			"EventTitle":       event.Title,
			"EventDescription": event.Description,
			"IsAuthenticated":  false,
			"Error":            r.URL.Query().Get("error"),
		})
		return
	}

	participants := s.store.ListParticipants()
	sort.Slice(participants, func(i, j int) bool {
		return participants[i].RegisteredAt.Before(participants[j].RegisteredAt)
	})

	type participantView struct {
		models.Participant
		ReceiverName string
	}

	pViews := make([]participantView, len(participants))
	pMap := make(map[string]models.Participant)
	for _, p := range participants {
		pMap[p.ID] = p
	}

	for i, p := range participants {
		recName := "Sin asignar"
		if p.ReceiverID != "" {
			if rec, ok := pMap[p.ReceiverID]; ok {
				recName = rec.Name
			}
		}
		pViews[i] = participantView{
			Participant:  p,
			ReceiverName: recName,
		}
	}

	codes := s.store.ListInviteCodes()
	sort.Slice(codes, func(i, j int) bool {
		return codes[i].CreatedAt.Before(codes[j].CreatedAt)
	})

	type codeView struct {
		models.InviteCode
		ParticipantName string
		InviteURL       string
	}

	cViews := make([]codeView, len(codes))
	unusedCount := 0
	for i, c := range codes {
		pName := ""
		if c.Used && c.ParticipantID != "" {
			if p, ok := pMap[c.ParticipantID]; ok {
				pName = p.Name
			}
		} else {
			unusedCount++
		}
		cViews[i] = codeView{
			InviteCode:      c,
			ParticipantName: pName,
			InviteURL:       s.cfg.FormatInviteLink(c.Code),
		}
	}

	s.render(w, "admin.html", map[string]any{
		"Title":            "Panel del Organizador",
		"EventTitle":       event.Title,
		"EventDescription": event.Description,
		"IsAuthenticated":  true,
		"Event":            event,
		"Participants":     pViews,
		"InviteCodes":      cViews,
		"Stats": map[string]int{
			"ParticipantCount": len(participants),
			"TotalCodes":       len(codes),
			"UnusedCodes":      unusedCount,
		},
		"Error":   r.URL.Query().Get("error"),
		"Success": r.URL.Query().Get("success"),
	})
}

// handleAdminLogout logs out the admin
func (s *Server) handleAdminLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieAdminSession,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	http.Redirect(w, r, "/admin?success=Sesión+de+organizador+cerrada.", http.StatusSeeOther)
}

// handleAdminGenerateCodes generates new invite codes
func (s *Server) handleAdminGenerateCodes(w http.ResponseWriter, r *http.Request) {
	countStr := r.FormValue("count")
	count, err := strconv.Atoi(countStr)
	if err != nil || count <= 0 {
		count = 5
	}
	if count > 100 {
		count = 100
	}

	newCodes, err := s.store.GenerateInviteCodes(count)
	if err != nil {
		http.Redirect(w, r, "/admin?error="+url.QueryEscape("Error al generar códigos: "+err.Error()), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/admin?success=¡Se+generaron+con+éxito+%d+enlaces+de+invitación+nuevos!", len(newCodes)), http.StatusSeeOther)
}

// handleAdminRoll executes the draw!
func (s *Server) handleAdminRoll(w http.ResponseWriter, r *http.Request) {
	participants := s.store.ListParticipants()

	assignments, err := logic.PerformDraw(participants)
	if err != nil {
		http.Redirect(w, r, "/admin?error="+url.QueryEscape("Error en el sorteo: "+err.Error()), http.StatusSeeOther)
		return
	}

	if err := s.store.SaveDrawResults(assignments); err != nil {
		http.Redirect(w, r, "/admin?error="+url.QueryEscape("Error al guardar los resultados: "+err.Error()), http.StatusSeeOther)
		return
	}

	log.Printf("[SORTEO REALIZADO] ¡%d participantes asignados con éxito!", len(assignments))

	http.Redirect(w, r, "/admin?success=¡Sorteo+del+intercambio+realizado+con+éxito!+Los+participantes+ya+pueden+ver+a+quién+les+tocó.", http.StatusSeeOther)
}

// handleAdminResetDraw resets the draw to allow re-registering or re-drawing
func (s *Server) handleAdminResetDraw(w http.ResponseWriter, r *http.Request) {
	if err := s.store.ResetDraw(); err != nil {
		http.Redirect(w, r, "/admin?error="+url.QueryEscape("Error al reiniciar el sorteo: "+err.Error()), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin?success=El+sorteo+ha+sido+reiniciado.+El+registro+está+abierto+nuevamente.", http.StatusSeeOther)
}

// handleAdminDeleteParticipant removes a participant before the draw
func (s *Server) handleAdminDeleteParticipant(w http.ResponseWriter, r *http.Request) {
	pID := r.FormValue("participant_id")
	if err := s.store.DeleteParticipant(pID); err != nil {
		http.Redirect(w, r, "/admin?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin?success=Participante+eliminado+y+código+de+invitación+restaurado.", http.StatusSeeOther)
}

// handleAdminUpdateEvent updates editable event metadata
func (s *Server) handleAdminUpdateEvent(w http.ResponseWriter, r *http.Request) {
	title := r.FormValue("title")
	desc := r.FormValue("description")
	budget := r.FormValue("budget")

	if err := s.store.UpdateEvent(title, desc, budget); err != nil {
		http.Redirect(w, r, "/admin?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin?success=¡Configuración+del+intercambio+actualizada+con+éxito!", http.StatusSeeOther)
}
