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
	"time"

	"secret_santa/config"
	"secret_santa/logic"
	"secret_santa/models"
	"secret_santa/storage"
)

type Server struct {
	cfg        *config.Config
	store      *storage.Store
	mailer     *logic.Mailer
	templates  map[string]*template.Template
	staticFS   http.FileSystem
}

// NewServer initializes the server with templates and handlers
func NewServer(cfg *config.Config, store *storage.Store, tmplFS embed.FS, staticFS http.FileSystem) (*Server, error) {
	pages := []string{"index.html", "join.html", "reveal.html", "admin.html"}
	templates := make(map[string]*template.Template)

	for _, page := range pages {
		t, err := template.ParseFS(tmplFS, "templates/layout.html", "templates/"+page)
		if err != nil {
			return nil, fmt.Errorf("failed to parse template %s: %w", page, err)
		}
		templates[page] = t
	}

	return &Server{
		cfg:       cfg,
		store:     store,
		mailer:    logic.NewMailer(cfg),
		templates: templates,
		staticFS:  staticFS,
	}, nil
}

// RegisterRoutes sets up all HTTP routes
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	// Static assets
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(s.staticFS)))

	// Public Participant Flows
	mux.HandleFunc("GET /{$}", s.handleHome)
	mux.HandleFunc("GET /code-redirect", s.handleCodeRedirect)
	mux.HandleFunc("GET /join/{code}", s.handleJoinPage)
	mux.HandleFunc("POST /join/{code}", s.handleJoinSubmit)
	mux.HandleFunc("GET /reveal/{code}", s.handleRevealPage)

	// Admin Dashboard & Actions
	mux.HandleFunc("GET /admin", s.handleAdminDashboard)
	mux.HandleFunc("POST /admin/login", s.handleAdminLogin)
	mux.HandleFunc("GET /admin/logout", s.handleAdminLogout)
	mux.HandleFunc("POST /admin/generate-codes", s.requireAdmin(s.handleAdminGenerateCodes))
	mux.HandleFunc("POST /admin/roll", s.requireAdmin(s.handleAdminRoll))
	mux.HandleFunc("POST /admin/resend-emails", s.requireAdmin(s.handleAdminResendEmails))
	mux.HandleFunc("POST /admin/reset-draw", s.requireAdmin(s.handleAdminResetDraw))
	mux.HandleFunc("POST /admin/delete-participant", s.requireAdmin(s.handleAdminDeleteParticipant))
	mux.HandleFunc("POST /admin/update-event", s.requireAdmin(s.handleAdminUpdateEvent))
}

// render renders a full page using layout.html and the specific sub-template
func (s *Server) render(w http.ResponseWriter, tmplName string, data any) {
	tmpl, ok := s.templates[tmplName]
	if !ok {
		log.Printf("Template not found: %s", tmplName)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Execute layout template with defined content
	err := tmpl.ExecuteTemplate(w, "layout", data)
	if err != nil {
		log.Printf("Template render error (%s): %v", tmplName, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleHome renders the welcome / code entry page
func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	event := s.store.GetEvent()
	s.render(w, "index.html", map[string]any{
		"Title":            "Welcome",
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
		http.Redirect(w, r, "/?error=Please+enter+an+invitation+code", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/join/"+url.PathEscape(code), http.StatusSeeOther)
}

// handleJoinPage shows the registration form or redirects to reveal if already registered
func (s *Server) handleJoinPage(w http.ResponseWriter, r *http.Request) {
	code := strings.ToUpper(strings.TrimSpace(r.PathValue("code")))
	event := s.store.GetEvent()

	inv, exists := s.store.GetInviteCode(code)
	if !exists {
		s.render(w, "index.html", map[string]any{
			"Title":            "Invalid Code",
			"EventTitle":       event.Title,
			"EventDescription": event.Description,
			"Error":            fmt.Sprintf("Invitation code '%s' is not valid. Please check your invitation link.", code),
		})
		return
	}

	// If the code was already redeemed, redirect directly to reveal page for this participant
	if inv.Used {
		http.Redirect(w, r, "/reveal/"+url.PathEscape(code), http.StatusSeeOther)
		return
	}

	// If draw already occurred, registration is closed
	if event.Status == models.StatusDrawn {
		s.render(w, "index.html", map[string]any{
			"Title":            "Registration Closed",
			"EventTitle":       event.Title,
			"EventDescription": event.Description,
			"Error":            "The Secret Santa draw has already taken place! Registration is now closed.",
		})
		return
	}

	s.render(w, "join.html", map[string]any{
		"Title":            "Register",
		"EventTitle":       event.Title,
		"EventDescription": event.Description,
		"Code":             code,
		"Budget":           event.Budget,
		"Error":            r.URL.Query().Get("error"),
		"FormName":         r.URL.Query().Get("name"),
		"FormEmail":        r.URL.Query().Get("email"),
		"FormWishlist":     r.URL.Query().Get("wishlist"),
	})
}

// handleJoinSubmit processes the registration form submission
func (s *Server) handleJoinSubmit(w http.ResponseWriter, r *http.Request) {
	code := strings.ToUpper(strings.TrimSpace(r.PathValue("code")))
	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.TrimSpace(r.FormValue("email"))
	wishlist := strings.TrimSpace(r.FormValue("wishlist"))

	if name == "" {
		redirectWithError(w, r, fmt.Sprintf("/join/%s", code), "Name is required", name, email, wishlist)
		return
	}

	if email == "" {
		redirectWithError(w, r, fmt.Sprintf("/join/%s", code), "Email address is required for match notification", name, email, wishlist)
		return
	}

	participant, err := s.store.RegisterParticipant(code, name, email, wishlist)
	if err != nil {
		redirectWithError(w, r, fmt.Sprintf("/join/%s", code), err.Error(), name, email, wishlist)
		return
	}

	log.Printf("[REGISTERED] Participant %s (%s) registered with code %s", participant.Name, participant.Email, code)

	http.Redirect(w, r, fmt.Sprintf("/reveal/%s?success=Registration+successful!+Welcome+to+Secret+Santa!", url.PathEscape(code)), http.StatusSeeOther)
}

func redirectWithError(w http.ResponseWriter, r *http.Request, targetURL, errMsg, name, email, wishlist string) {
	q := url.Values{}
	q.Set("error", errMsg)
	if name != "" {
		q.Set("name", name)
	}
	if email != "" {
		q.Set("email", email)
	}
	if wishlist != "" {
		q.Set("wishlist", wishlist)
	}
	http.Redirect(w, r, targetURL+"?"+q.Encode(), http.StatusSeeOther)
}

// handleRevealPage displays the participant's status or their drawn match
func (s *Server) handleRevealPage(w http.ResponseWriter, r *http.Request) {
	code := strings.ToUpper(strings.TrimSpace(r.PathValue("code")))
	event := s.store.GetEvent()

	participant, exists := s.store.GetParticipantByCode(code)
	if !exists {
		// Code might not be registered yet
		inv, invExists := s.store.GetInviteCode(code)
		if invExists && !inv.Used {
			http.Redirect(w, r, "/join/"+url.PathEscape(code), http.StatusSeeOther)
			return
		}
		s.render(w, "index.html", map[string]any{
			"Title":            "Not Found",
			"EventTitle":       event.Title,
			"EventDescription": event.Description,
			"Error":            "No participant registration found for this invitation code.",
		})
		return
	}

	var receiver models.Participant
	if event.Status == models.StatusDrawn && participant.ReceiverID != "" {
		rec, found := s.store.GetParticipant(participant.ReceiverID)
		if found {
			receiver = rec
		}
	}

	s.render(w, "reveal.html", map[string]any{
		"Title":            fmt.Sprintf("Secret Santa - %s", participant.Name),
		"EventTitle":       event.Title,
		"EventDescription": event.Description,
		"Status":           event.Status,
		"Participant":      participant,
		"Receiver":         receiver,
		"Budget":           event.Budget,
		"Success":          r.URL.Query().Get("success"),
	})
}

// Admin Authentication Helpers
func (s *Server) isAdminAuthenticated(r *http.Request) bool {
	event := s.store.GetEvent()
	if event.AdminKey == "" {
		return false
	}

	// Check URL param ?key=...
	if key := r.URL.Query().Get("key"); key != "" {
		if subtle.ConstantTimeCompare([]byte(key), []byte(event.AdminKey)) == 1 {
			return true
		}
	}

	// Check Cookie
	cookie, err := r.Cookie("admin_auth")
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
			http.Redirect(w, r, "/admin?error=Unauthorized.+Please+log+in+with+the+admin+passkey.", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

// handleAdminDashboard renders organizer control panel
func (s *Server) handleAdminDashboard(w http.ResponseWriter, r *http.Request) {
	event := s.store.GetEvent()
	isAuth := s.isAdminAuthenticated(r)

	// If URL had ?key=..., set cookie automatically
	if key := r.URL.Query().Get("key"); key != "" && isAuth {
		http.SetCookie(w, &http.Cookie{
			Name:     "admin_auth",
			Value:    event.AdminKey,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   86400 * 7, // 7 days
		})
	}

	if !isAuth {
		s.render(w, "admin.html", map[string]any{
			"Title":            "Admin Login",
			"EventTitle":       event.Title,
			"EventDescription": event.Description,
			"IsAuthenticated":  false,
			"Error":            r.URL.Query().Get("error"),
		})
		return
	}

	// Gather stats & list of participants and codes
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
		recName := "Not assigned"
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
		"Title":            "Admin Dashboard",
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

// handleAdminLogin authenticates passkey and sets cookie
func (s *Server) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimSpace(r.FormValue("admin_key"))
	event := s.store.GetEvent()

	if subtle.ConstantTimeCompare([]byte(key), []byte(event.AdminKey)) != 1 {
		http.Redirect(w, r, "/admin?error=Invalid+admin+passkey", http.StatusSeeOther)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "admin_auth",
		Value:    event.AdminKey,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400 * 7,
	})

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

// handleAdminLogout logs out the admin
func (s *Server) handleAdminLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "admin_auth",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
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
		http.Redirect(w, r, "/admin?error="+url.QueryEscape("Failed to generate codes: "+err.Error()), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/admin?success=Successfully+generated+%d+new+invitation+links!", len(newCodes)), http.StatusSeeOther)
}

// handleAdminRoll executes the Secret Santa draw!
func (s *Server) handleAdminRoll(w http.ResponseWriter, r *http.Request) {
	participants := s.store.ListParticipants()

	assignments, err := logic.PerformDraw(participants)
	if err != nil {
		http.Redirect(w, r, "/admin?error="+url.QueryEscape("Draw failed: "+err.Error()), http.StatusSeeOther)
		return
	}

	if err := s.store.SaveDrawResults(assignments); err != nil {
		http.Redirect(w, r, "/admin?error="+url.QueryEscape("Failed to save draw results: "+err.Error()), http.StatusSeeOther)
		return
	}

	// Trigger notifications in background goroutine
	go s.dispatchAllMatchEmails()

	http.Redirect(w, r, "/admin?success=Secret+Santa+draw+successfully+completed!+Notifications+dispatched.", http.StatusSeeOther)
}

// dispatchAllMatchEmails sends notification emails to all participants
func (s *Server) dispatchAllMatchEmails() {
	event := s.store.GetEvent()
	participants := s.store.ListParticipants()

	pMap := make(map[string]models.Participant)
	for _, p := range participants {
		pMap[p.ID] = p
	}

	for _, giver := range participants {
		if giver.ReceiverID == "" {
			continue
		}
		receiver, ok := pMap[giver.ReceiverID]
		if !ok {
			continue
		}

		err := s.mailer.SendMatchNotification(event, giver, receiver)
		if err != nil {
			log.Printf("[EMAIL ERROR] Failed sending match to %s (%s): %v", giver.Name, giver.Email, err)
			_ = s.store.UpdateParticipantEmailStatus(giver.ID, false, err.Error())
		} else {
			log.Printf("[EMAIL SENT] Successfully notified %s (%s) about their match", giver.Name, giver.Email)
			_ = s.store.UpdateParticipantEmailStatus(giver.ID, true, "")
		}
		time.Sleep(50 * time.Millisecond) // Polite delay between emails
	}
}

// handleAdminResendEmails manually triggers resending match emails
func (s *Server) handleAdminResendEmails(w http.ResponseWriter, r *http.Request) {
	event := s.store.GetEvent()
	if event.Status != models.StatusDrawn {
		http.Redirect(w, r, "/admin?error=Cannot+send+emails+before+the+draw+has+been+rolled", http.StatusSeeOther)
		return
	}

	go s.dispatchAllMatchEmails()
	http.Redirect(w, r, "/admin?success=Email+notifications+queued+for+delivery!", http.StatusSeeOther)
}

// handleAdminResetDraw resets the draw to allow re-registering or re-drawing
func (s *Server) handleAdminResetDraw(w http.ResponseWriter, r *http.Request) {
	if err := s.store.ResetDraw(); err != nil {
		http.Redirect(w, r, "/admin?error="+url.QueryEscape("Failed to reset draw: "+err.Error()), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin?success=Draw+has+been+reset.+Registration+is+re-opened.", http.StatusSeeOther)
}

// handleAdminDeleteParticipant removes a participant before the draw
func (s *Server) handleAdminDeleteParticipant(w http.ResponseWriter, r *http.Request) {
	pID := r.FormValue("participant_id")
	if err := s.store.DeleteParticipant(pID); err != nil {
		http.Redirect(w, r, "/admin?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin?success=Participant+removed+and+invite+code+restored.", http.StatusSeeOther)
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

	http.Redirect(w, r, "/admin?success=Event+settings+updated+successfully!", http.StatusSeeOther)
}
