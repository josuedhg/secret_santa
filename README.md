# 🎅 Secret Santa Application

A modern, standalone Secret Santa web application built with Go. It features single-use invitation links, duplicate name prevention, cryptographically randomized draw matching, personal match reveal pages with interactive unwrap animations, and optional email notifications.

---

## ✨ Key Features & Rules Enforced

- **Single-Use Invitation Links**:
  - The organizer generates unique, single-use invite links (e.g. `http://localhost:8080/join/SANTA-ABCD-1234`).
  - Once a user registers with a link, that code is locked to them and cannot be reused by anyone else.
  - Re-visiting the code redirects directly to the participant's personal reveal dashboard.

- **Rule 1: No Self-Assignments**:
  - The draw algorithm mathematically guarantees that no one can ever draw their own name ($Giver \ne Receiver$).

- **Rule 2: No Duplicate Names**:
  - Names are validated case-insensitively during registration. If a name is already taken, the registrant is prompted to provide their full name or unique nickname.

- **Rule 3: Everyone Gives & Receives Exactly 1 Gift**:
  - The draw forms a single unified Hamiltonian cycle ($P_0 \to P_1 \to \dots \to P_{n-1} \to P_0$) shuffled with cryptographic randomness (`crypto/rand`). This also prevents small disjoint 2-cycles (A $\leftrightarrow$ B) for groups of 3 or more.

- **Dual Notification System**:
  - **Personal Secret Reveal Link**: Participants can visit their private link anytime to reveal their recipient with an interactive unwrap card and confetti effect.
  - **Email Notifications**: Automatically dispatches festive HTML and plain text emails to each participant with their recipient's name and wishlist. If SMTP is not configured, it runs in safe mock mode and logs to the console.

- **Organizer Admin Dashboard**:
  - Protected by a secret admin passkey.
  - Generate 1, 5, 10, or custom batches of invite links.
  - One-click copy for individual links or "📋 Copy All Unused Links" for sharing in group chats.
  - Real-time participant tracker with optional wishlist hints.
  - Safe pre-draw participant removal (automatically restores the invite code).
  - Roll Secret Santa draw button with confirmation and participant safety checks.

---

## 🚀 Quick Start

### 1. Build and Run

With Go installed (Go 1.22+ recommended):

```bash
# Clone and enter directory
cd secret_santa

# Build binary
go build -o secret-santa .

# Run the application
./secret-santa
```

Or run directly with `go run`:

```bash
go run .
```

On startup, the console displays:
- The base web address (e.g. `http://localhost:8080`)
- The generated Admin Passkey
- The direct admin access URL (e.g. `http://localhost:8080/admin?key=...`)

---

## ⚙️ Configuration & Flags

All settings can be configured via command-line flags or environment variables:

| Flag | Environment Variable | Default | Description |
|------|----------------------|---------|-------------|
| `-port` | `PORT` | `8080` | HTTP port to listen on |
| `-base-url` | `BASE_URL` | `http://localhost:8080` | Base URL used when formatting invitation and reveal links |
| `-data` | `DATA_PATH` | `data/secretsanta.json` | Path to persistent JSON database file |
| `-admin-key` | `ADMIN_KEY` | *(Auto-generated)* | Admin secret passkey |
| `-smtp-host` | `SMTP_HOST` | `""` | SMTP server address (e.g. `smtp.gmail.com` or `smtp.resend.com`) |
| `-smtp-port` | `SMTP_PORT` | `587` | SMTP port (`587` for STARTTLS, `465` for SSL/TLS) |
| `-smtp-user` | `SMTP_USER` | `""` | SMTP username |
| `-smtp-pass` | `SMTP_PASS` | `""` | SMTP password / app password |
| `-smtp-from` | `SMTP_FROM` | `""` | Sender email address (e.g. `santa@yourdomain.com`) |

### Example with Custom Admin Key and Public Domain:

```bash
./secret-santa -port 3000 -base-url "https://santa.mycompany.com" -admin-key "super-secret-key"
```

### Example with Email (SMTP):

```bash
SMTP_HOST="smtp.gmail.com" \
SMTP_PORT=587 \
SMTP_USER="organizer@gmail.com" \
SMTP_PASS="your-app-password" \
SMTP_FROM="Secret Santa <organizer@gmail.com>" \
./secret-santa
```

*(Note: If SMTP is omitted, the app will log simulated emails to the terminal, and users can always view their assignment via their personal reveal link).*

---

## 📖 Organizer Walkthrough

1. Open the **Admin Dashboard** using the link printed in the terminal or navigate to `/admin` and enter your Admin Passkey.
2. Under **Invitation Links**, generate as many invite codes as you have participants.
3. Click **"Copy All Unused Links"** or copy individual links and send one to each participant (via WhatsApp, Slack, Discord, Email, SMS, etc.).
4. Participants click their link, enter their name (enforcing no duplicates), email, and an optional wishlist or gift preferences.
5. Watch the participant count update on the dashboard.
6. Once everyone has joined, click **"🎲 Roll Secret Santa Now!"**.
7. Matches are assigned, emails are dispatched, and participants can click **"Unwrap Your Match"** on their personal link!

---

## 🧪 Testing

Run all unit tests and end-to-end integration tests:

```bash
go test -v ./...
```

Tests cover:
- Cryptographic shuffle and single-cycle derangement algorithm.
- Exclusion of self-assignments ($Giver \ne Receiver$).
- Strict 1-to-1 bijection (everyone gives 1 gift and receives 1 gift).
- Duplicate name rejection.
- Single-use invite code token enforcement.
- Persistence and atomic file writing.
- Full HTTP flow from registration to admin roll and match reveal.

---

## 📁 Project Structure

```
secret_santa/
├── main.go               # Application entrypoint, CLI flags, graceful shutdown
├── main_test.go          # Full end-to-end integration tests
├── config/
│   └── config.go         # Configuration loading and link formatters
├── models/
│   └── models.go         # Event, Participant, and InviteCode data structures
├── storage/
│   ├── storage.go        # Thread-safe persistent JSON store with atomic writes
│   └── storage_test.go   # Store unit tests
├── logic/
│   ├── draw.go           # Secret Santa matching algorithm & validation
│   ├── draw_test.go      # Draw algorithm tests (invariance, cycle, zero self-match)
│   └── email.go          # SMTP multipart (HTML + Text) notification mailer
├── handlers/
│   └── handlers.go       # HTTP request handlers & admin security
├── templates/
│   ├── layout.html       # Base HTML template with festive styling
│   ├── index.html        # Welcome / invite code input
│   ├── join.html         # Participant registration form
│   ├── reveal.html       # Match reveal page with interactive gift box
│   └── admin.html        # Organizer admin dashboard
└── static/
    ├── app.css           # Modern festive responsive stylesheet
    └── app.js            # Clipboard copy & confetti unwrap animations
```
