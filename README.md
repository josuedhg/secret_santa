# 🎅 Secret Santa Application (Zero Email & Zero Password)

A simple, frictionless Secret Santa web application built with Go. **No passwords and no email accounts are required.** 

Participants simply open their unique one-time invitation link, enter their name, and a persistent browser session is established. Whenever they return to the website, they stay logged in and can view their match as soon as the organizer rolls the Secret Santa draw!

---

## ✨ How It Works (Frictionless Flow)

1. **Organizer creates unique invitation links**:
   - The organizer accesses `/admin?key=<organizer_key>` (which permanently saves their organizer session on their browser).
   - Generates unique, single-use invite links (e.g. `http://localhost:8080/join/SANTA-ABCD-1234`).
   - Copies individual links or clicks **"Copy All Unused Links"** to send to participants via WhatsApp, Slack, Discord, SMS, etc.

2. **Participants join with 1 click & stay logged in**:
   - The participant opens their link.
   - Enters their **Name** (duplicate names are prevented) and optional **Wishlist / Gift Hints**.
   - **No email and no password required.**
   - A persistent session cookie (`santa_participant_session`) is created for 1 year.
   - Any time they revisit the website (`/` or `/reveal`) on that browser, it automatically recognizes them!

3. **Organizer rolls the draw**:
   - When everyone has joined, the organizer clicks **"🎲 Roll Secret Santa Now!"**.
   - The algorithm executes a single Hamiltonian cycle shuffle:
     - **Rule 1**: No one gets their own name ($Giver \ne Receiver$).
     - **Rule 2**: Names cannot be duplicated.
     - **Rule 3**: Everyone gives exactly 1 gift and receives exactly 1 gift.

4. **Participants unwrap their gift match**:
   - Participants simply open the website on their phone or computer.
   - They click the interactive **"🎁 Click to Unwrap Your Match"** gift box card with confetti to reveal their recipient!

---

## 🚀 Quick Start

### Run with Go:

```bash
cd secret_santa
go run .
```

Or build the standalone single binary:

```bash
go build -o secret-santa .
./secret-santa
```

The server displays:
```text
==================================================================
🎅 SECRET SANTA APPLICATION (No Passwords & No Emails Required)
==================================================================
 Server running at:     http://localhost:8080
 Data stored at:        data/secretsanta.json
 Organizer Key:         SANTA-PASSKEY-XXXX
 Direct Organizer Link: http://localhost:8080/admin?key=SANTA-PASSKEY-XXXX
==================================================================
```

---

## ⚙️ Configuration & Flags

| Flag | Environment Variable | Default | Description |
|------|----------------------|---------|-------------|
| `-port` | `PORT` | `8080` | HTTP port to listen on |
| `-base-url` | `BASE_URL` | `http://localhost:8080` | Public base URL used when generating invite links |
| `-data` | `DATA_PATH` | `data/secretsanta.json` | Path to persistent JSON database file |
| `-admin-key` | `ADMIN_KEY` | *(Auto-generated)* | Organizer secret key |

---

## 🧪 Testing

Run all unit tests and end-to-end integration tests:

```bash
go test -v ./...
```
