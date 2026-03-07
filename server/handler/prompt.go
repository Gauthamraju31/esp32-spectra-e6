package handler

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Gauthamraju31/esp32-spectra-e6/server/auth"
	"github.com/Gauthamraju31/esp32-spectra-e6/server/dither"
	"github.com/Gauthamraju31/esp32-spectra-e6/server/provider"
	"github.com/Gauthamraju31/esp32-spectra-e6/server/ratelimit"
	"github.com/Gauthamraju31/esp32-spectra-e6/server/store"
)

// PromptHandler handles the prompt page and image generation.
type PromptHandler struct {
	auth          *auth.Manager
	limiter       *ratelimit.Limiter
	provider      provider.ImageProvider
	ditherer      *dither.Ditherer
	store         store.ImageStore
	templates     *template.Template
	displayWidth  int
	displayHeight int
	cdnDomain     string
}

// NewPromptHandler creates a new prompt handler.
func NewPromptHandler(
	authMgr *auth.Manager,
	limiter *ratelimit.Limiter,
	prov provider.ImageProvider,
	d *dither.Ditherer,
	s store.ImageStore,
	displayWidth, displayHeight int,
	cdnDomain string,
) *PromptHandler {
	tmpl := template.Must(template.New("").Parse(loginTemplate + promptTemplate))
	return &PromptHandler{
		auth:          authMgr,
		limiter:       limiter,
		provider:      prov,
		ditherer:      d,
		store:         s,
		templates:     tmpl,
		displayWidth:  displayWidth,
		displayHeight: displayHeight,
		cdnDomain:     cdnDomain,
	}
}

type loginData struct {
	Error string
}

type promptData struct {
	ProviderName  string
	Remaining     int
	Limit             int
	HasImage          bool
	UpdatedAt         string
	HasFirmware       bool
	FirmwareUpdatedAt string
	Success           string
	Error         string
	Prompt        string
	Orientation   string
	DisplayWidth  int
	DisplayHeight int
	CdnDomain     string
}

// HandleLogin renders the login page.
func (h *PromptHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// If already authenticated, redirect to prompt page
		token := auth.GetSessionToken(r)
		if h.auth.ValidateSession(token) {
			http.Redirect(w, r, "/prompt", http.StatusSeeOther)
			return
		}
		h.templates.ExecuteTemplate(w, "login", loginData{})
		return
	}

	// POST — validate password
	password := r.FormValue("password")
	if !h.auth.CheckPassword(password) {
		w.WriteHeader(http.StatusUnauthorized)
		h.templates.ExecuteTemplate(w, "login", loginData{Error: "Invalid password"})
		return
	}

	token, err := h.auth.CreateSession()
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	auth.SetSessionCookie(w, token)
	http.Redirect(w, r, "/prompt", http.StatusSeeOther)
}

// HandleLogout clears the session cookie.
func (h *PromptHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// HandlePrompt renders the prompt page and handles image generation.
func (h *PromptHandler) HandlePrompt(w http.ResponseWriter, r *http.Request) {
	data := promptData{
		ProviderName:  h.provider.Name(),
		Remaining:     h.limiter.Remaining(),
		Limit:         h.limiter.Limit(),
		HasImage:      h.store.HasImage(),
		HasFirmware:   h.store.HasFirmware(),
		Orientation:   "landscape",
		DisplayWidth:  h.displayWidth,
		DisplayHeight: h.displayHeight,
		CdnDomain:     h.cdnDomain,
	}

	if h.store.HasImage() {
		data.UpdatedAt = h.store.UpdatedAt().Format(time.RFC822)
	}
	if h.store.HasFirmware() {
		data.FirmwareUpdatedAt = h.store.FirmwareUpdatedAt().Format(time.RFC822)
	}

	if r.Method == http.MethodGet {
		if r.URL.Query().Get("success") == "firmware" {
			data.Success = "Firmware successfully uploaded to storage."
		}
		if errCode := r.URL.Query().Get("error"); errCode != "" {
			switch errCode {
			case "upload_too_large":
				data.Error = "Firmware file is too large (maximum 4 MB)."
			case "no_file":
				data.Error = "No firmware file was provided."
			case "read_error":
				data.Error = "Failed to read the uploaded firmware file."
			case "save_error":
				data.Error = "Failed to securely save the firmware to storage."
			default:
				data.Error = "Failed to upload firmware."
			}
		}
		h.templates.ExecuteTemplate(w, "prompt", data)
		return
	}

	// POST — generate image
	prompt := strings.TrimSpace(r.FormValue("prompt"))
	orientation := r.FormValue("orientation")
	data.Prompt = prompt
	if orientation == "portrait" {
		data.Orientation = "portrait"
	}

	if prompt == "" {
		data.Error = "Please enter a prompt."
		h.templates.ExecuteTemplate(w, "prompt", data)
		return
	}

	if !h.limiter.Allow() {
		data.Error = "Daily rate limit reached. Try again tomorrow."
		data.Remaining = 0
		h.templates.ExecuteTemplate(w, "prompt", data)
		return
	}

	// Generate image
	log.Printf("Generating image with %s: %q", h.provider.Name(), prompt)

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	// Attach desired image dimensions to context (orientation-aware).
	imgW, imgH := h.displayWidth, h.displayHeight
	if data.Orientation == "portrait" {
		imgW, imgH = h.displayHeight, h.displayWidth
	}
	ctx = provider.WithImageDims(ctx, imgW, imgH)

	imgData, contentType, err := h.provider.Generate(ctx, prompt)
	if err != nil {
		data.Error = fmt.Sprintf("Image generation failed: %v", err)
		data.Remaining = h.limiter.Remaining()
		h.templates.ExecuteTemplate(w, "prompt", data)
		return
	}

	log.Printf("Generated image: %d bytes (%s)", len(imgData), contentType)

	// Save original for preview
	if err := h.store.SaveOriginal(imgData, contentType); err != nil {
		log.Printf("Warning: failed to save original image: %v", err)
	}

	// Dither for e-paper
	ditherW, ditherH := h.displayWidth, h.displayHeight
	if data.Orientation == "portrait" {
		ditherW, ditherH = h.displayHeight, h.displayWidth
	}
	log.Printf("Dithering image for e-paper display (%s: %d×%d)...", data.Orientation, ditherW, ditherH)
	bmpData, err := h.ditherer.ProcessWithSize(imgData, ditherW, ditherH)
	if err != nil {
		data.Error = fmt.Sprintf("Dithering failed: %v", err)
		data.Remaining = h.limiter.Remaining()
		h.templates.ExecuteTemplate(w, "prompt", data)
		return
	}

	log.Printf("Dithered BMP: %d bytes", len(bmpData))

	// Save dithered image
	if err := h.store.Save(bmpData); err != nil {
		data.Error = fmt.Sprintf("Failed to save image: %v", err)
		data.Remaining = h.limiter.Remaining()
		h.templates.ExecuteTemplate(w, "prompt", data)
		return
	}

	data.Success = "Image generated and ready for display!"
	data.HasImage = true
	data.Remaining = h.limiter.Remaining()
	data.UpdatedAt = time.Now().Format(time.RFC822)
	h.templates.ExecuteTemplate(w, "prompt", data)
}

// HandleFirmwareUpload handles uploading a .bin file for OTA updates.
func (h *PromptHandler) HandleFirmwareUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/prompt", http.StatusSeeOther)
		return
	}

	// Security: Enforce a strict 4MB upload limit to prevent memory/disk exhaustion attacks.
	// Most ESP32 OTA partitions are well under 4MB.
	const maxUploadSize = 4 << 20 // 4 MB
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

	err := r.ParseMultipartForm(maxUploadSize)
	if err != nil {
		if err.Error() == "http: request body too large" {
			http.Redirect(w, r, "/prompt?error=upload_too_large", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/prompt?error=upload_failed", http.StatusSeeOther)
		return
	}

	file, _, err := r.FormFile("firmware")
	if err != nil {
		http.Redirect(w, r, "/prompt?error=no_file", http.StatusSeeOther)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		http.Redirect(w, r, "/prompt?error=read_error", http.StatusSeeOther)
		return
	}

	if err := h.store.SaveFirmware(data); err != nil {
		log.Printf("Failed to save firmware: %v", err)
		http.Redirect(w, r, "/prompt?error=save_error", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/prompt?success=firmware", http.StatusSeeOther)
}

// HandleRoot redirects to the appropriate page.
func (h *PromptHandler) HandleRoot(w http.ResponseWriter, r *http.Request) {
	token := auth.GetSessionToken(r)
	if h.auth.ValidateSession(token) {
		http.Redirect(w, r, "/prompt", http.StatusSeeOther)
	} else {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}

const loginTemplate = `{{define "login"}}<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>E-Paper Studio — Login</title>
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&display=swap" rel="stylesheet">
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }

        body {
            font-family: 'Inter', -apple-system, BlinkMacSystemFont, sans-serif;
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            background: #0a0a0f;
            color: #e4e4e7;
        }

        .login-container {
            width: 100%;
            max-width: 420px;
            padding: 2rem;
        }

        .logo {
            text-align: center;
            margin-bottom: 2rem;
        }

        .logo-icon {
            width: 64px;
            height: 64px;
            margin: 0 auto 1rem;
            background: linear-gradient(135deg, #6366f1, #8b5cf6, #a855f7);
            border-radius: 16px;
            display: flex;
            align-items: center;
            justify-content: center;
            font-size: 28px;
        }

        .logo h1 {
            font-size: 1.5rem;
            font-weight: 600;
            letter-spacing: -0.02em;
            background: linear-gradient(135deg, #e4e4e7, #a1a1aa);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
        }

        .logo p {
            color: #71717a;
            font-size: 0.875rem;
            margin-top: 0.25rem;
        }

        .card {
            background: rgba(255, 255, 255, 0.03);
            border: 1px solid rgba(255, 255, 255, 0.06);
            border-radius: 16px;
            padding: 2rem;
            backdrop-filter: blur(20px);
        }

        .form-group {
            margin-bottom: 1.5rem;
        }

        .form-group label {
            display: block;
            font-size: 0.8rem;
            font-weight: 500;
            color: #a1a1aa;
            margin-bottom: 0.5rem;
            text-transform: uppercase;
            letter-spacing: 0.05em;
        }

        .form-group input {
            width: 100%;
            padding: 0.75rem 1rem;
            background: rgba(255, 255, 255, 0.04);
            border: 1px solid rgba(255, 255, 255, 0.08);
            border-radius: 10px;
            color: #e4e4e7;
            font-size: 1rem;
            font-family: inherit;
            transition: all 0.2s;
            outline: none;
        }

        .form-group input:focus {
            border-color: #6366f1;
            box-shadow: 0 0 0 3px rgba(99, 102, 241, 0.15);
        }

        .btn {
            width: 100%;
            padding: 0.75rem 1.5rem;
            background: linear-gradient(135deg, #6366f1, #7c3aed);
            color: white;
            border: none;
            border-radius: 10px;
            font-size: 0.95rem;
            font-weight: 500;
            font-family: inherit;
            cursor: pointer;
            transition: all 0.25s;
        }

        .btn:hover {
            transform: translateY(-1px);
            box-shadow: 0 8px 25px rgba(99, 102, 241, 0.3);
        }

        .btn:active { transform: translateY(0); }

        .error {
            background: rgba(239, 68, 68, 0.1);
            border: 1px solid rgba(239, 68, 68, 0.2);
            color: #fca5a5;
            padding: 0.75rem 1rem;
            border-radius: 10px;
            font-size: 0.875rem;
            margin-bottom: 1.5rem;
        }
    </style>
</head>
<body>
    <div class="login-container">
        <div class="logo">
            <div class="logo-icon">🖼</div>
            <h1>E-Paper Studio</h1>
            <p>Generate images for your e-paper display</p>
        </div>
        <div class="card">
            {{if .Error}}<div class="error">{{.Error}}</div>{{end}}
            <form method="POST" action="/login">
                <div class="form-group">
                    <label for="password">Password</label>
                    <input type="password" id="password" name="password" placeholder="Enter access password" required autofocus>
                </div>
                <button type="submit" class="btn">Sign In</button>
            </form>
        </div>
    </div>
</body>
</html>{{end}}`

const promptTemplate = `{{define "prompt"}}<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>E-Paper Studio — Prompt</title>
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&display=swap" rel="stylesheet">
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }

        body {
            font-family: 'Inter', -apple-system, BlinkMacSystemFont, sans-serif;
            min-height: 100vh;
            background: #0a0a0f;
            color: #e4e4e7;
        }

        .header {
            display: flex;
            align-items: center;
            justify-content: space-between;
            padding: 1rem 2rem;
            border-bottom: 1px solid rgba(255, 255, 255, 0.06);
            backdrop-filter: blur(20px);
            position: sticky;
            top: 0;
            z-index: 10;
            background: rgba(10, 10, 15, 0.8);
        }

        .header-left {
            display: flex;
            align-items: center;
            gap: 0.75rem;
        }

        .header-icon {
            width: 36px;
            height: 36px;
            background: linear-gradient(135deg, #6366f1, #8b5cf6);
            border-radius: 10px;
            display: flex;
            align-items: center;
            justify-content: center;
            font-size: 18px;
        }

        .header h1 {
            font-size: 1.1rem;
            font-weight: 600;
            letter-spacing: -0.01em;
        }

        .header-right {
            display: flex;
            align-items: center;
            gap: 1.25rem;
        }

        .rate-badge {
            display: flex;
            align-items: center;
            gap: 0.5rem;
            padding: 0.4rem 0.75rem;
            background: rgba(255, 255, 255, 0.04);
            border: 1px solid rgba(255, 255, 255, 0.08);
            border-radius: 20px;
            font-size: 0.8rem;
            color: #a1a1aa;
        }

        .rate-dot {
            width: 8px;
            height: 8px;
            border-radius: 50%;
            background: #22c55e;
        }

        .rate-dot.warning { background: #f59e0b; }
        .rate-dot.danger { background: #ef4444; }

        .logout-link {
            color: #71717a;
            text-decoration: none;
            font-size: 0.8rem;
            transition: color 0.2s;
        }

        .logout-link:hover { color: #e4e4e7; }

        .main {
            max-width: 900px;
            margin: 0 auto;
            padding: 2rem;
        }

        .card {
            background: rgba(255, 255, 255, 0.03);
            border: 1px solid rgba(255, 255, 255, 0.06);
            border-radius: 16px;
            padding: 1.5rem;
            backdrop-filter: blur(20px);
            margin-bottom: 1.5rem;
        }

        .card-title {
            font-size: 0.8rem;
            font-weight: 500;
            color: #a1a1aa;
            text-transform: uppercase;
            letter-spacing: 0.05em;
            margin-bottom: 1rem;
        }

        .prompt-area {
            width: 100%;
            min-height: 120px;
            padding: 1rem;
            background: rgba(255, 255, 255, 0.04);
            border: 1px solid rgba(255, 255, 255, 0.08);
            border-radius: 12px;
            color: #e4e4e7;
            font-size: 0.95rem;
            font-family: inherit;
            resize: vertical;
            transition: all 0.2s;
            outline: none;
            line-height: 1.5;
        }

        .prompt-area:focus {
            border-color: #6366f1;
            box-shadow: 0 0 0 3px rgba(99, 102, 241, 0.15);
        }

        .prompt-area::placeholder { color: #52525b; }

        .prompt-footer {
            display: flex;
            align-items: center;
            justify-content: space-between;
            margin-top: 1rem;
            gap: 1rem;
        }

        .provider-info {
            font-size: 0.8rem;
            color: #71717a;
        }

        .provider-name {
            color: #a1a1aa;
            font-weight: 500;
        }

        .btn {
            padding: 0.7rem 1.75rem;
            background: linear-gradient(135deg, #6366f1, #7c3aed);
            color: white;
            border: none;
            border-radius: 10px;
            font-size: 0.9rem;
            font-weight: 500;
            font-family: inherit;
            cursor: pointer;
            transition: all 0.25s;
            display: flex;
            align-items: center;
            gap: 0.5rem;
        }

        .btn:hover {
            transform: translateY(-1px);
            box-shadow: 0 8px 25px rgba(99, 102, 241, 0.3);
        }

        .btn:active { transform: translateY(0); }

        .btn:disabled {
            opacity: 0.5;
            cursor: not-allowed;
            transform: none;
            box-shadow: none;
        }

        .btn .spinner {
            display: none;
            width: 16px;
            height: 16px;
            border: 2px solid rgba(255, 255, 255, 0.3);
            border-top-color: white;
            border-radius: 50%;
            animation: spin 0.6s linear infinite;
        }

        .btn.loading .spinner { display: block; }
        .btn.loading .btn-text { display: none; }

        @keyframes spin { to { transform: rotate(360deg); } }

        .alert {
            padding: 0.75rem 1rem;
            border-radius: 10px;
            font-size: 0.875rem;
            margin-bottom: 1.25rem;
        }

        .alert.success {
            background: rgba(34, 197, 94, 0.1);
            border: 1px solid rgba(34, 197, 94, 0.2);
            color: #86efac;
        }

        .alert.error {
            background: rgba(239, 68, 68, 0.1);
            border: 1px solid rgba(239, 68, 68, 0.2);
            color: #fca5a5;
        }

        .preview-section {
            text-align: center;
        }

        .preview-grid {
            display: grid;
            grid-template-columns: 1fr 1fr;
            gap: 1.5rem;
            margin-bottom: 1rem;
        }

        .preview-item {
            text-align: center;
        }

        .preview-label {
            font-size: 0.75rem;
            font-weight: 500;
            color: #a1a1aa;
            text-transform: uppercase;
            letter-spacing: 0.05em;
            margin-bottom: 0.75rem;
        }

        .preview-container {
            position: relative;
            display: inline-block;
            border-radius: 12px;
            overflow: hidden;
            border: 1px solid rgba(255, 255, 255, 0.08);
            background: #1a1a2e;
        }

        .preview-container img {
            display: block;
            max-width: 100%;
            height: auto;
        }

        .preview-meta {
            display: flex;
            align-items: center;
            justify-content: center;
            gap: 1.5rem;
            margin-top: 0.75rem;
            font-size: 0.8rem;
            color: #71717a;
        }

        .no-image {
            padding: 3rem 2rem;
            text-align: center;
            color: #52525b;
        }

        .no-image-icon {
            font-size: 2.5rem;
            margin-bottom: 0.75rem;
            opacity: 0.5;
        }

        .orientation-row {
            display: flex;
            align-items: center;
            gap: 1rem;
            margin-top: 1rem;
        }

        .orientation-label {
            font-size: 0.8rem;
            font-weight: 500;
            color: #a1a1aa;
            text-transform: uppercase;
            letter-spacing: 0.05em;
            white-space: nowrap;
        }

        .orientation-toggle {
            display: flex;
            gap: 0;
            background: rgba(255, 255, 255, 0.04);
            border: 1px solid rgba(255, 255, 255, 0.08);
            border-radius: 10px;
            overflow: hidden;
        }

        .orientation-toggle input[type="radio"] {
            display: none;
        }

        .toggle-option {
            display: flex;
            align-items: center;
            gap: 0.4rem;
            padding: 0.45rem 0.85rem;
            font-size: 0.8rem;
            color: #71717a;
            cursor: pointer;
            transition: all 0.2s;
            border-right: 1px solid rgba(255, 255, 255, 0.06);
        }

        .toggle-option:last-of-type {
            border-right: none;
        }

        .orientation-toggle input[type="radio"]:checked + .toggle-option {
            background: rgba(99, 102, 241, 0.15);
            color: #a5b4fc;
        }

        .palette-dots {
            display: flex;
            justify-content: center;
            gap: 0.5rem;
            margin-top: 0.75rem;
        }

        .palette-dot {
            width: 14px;
            height: 14px;
            border-radius: 50%;
            border: 1px solid rgba(255, 255, 255, 0.1);
        }

        @media (max-width: 640px) {
            .main { padding: 1rem; }
            .header { padding: 0.75rem 1rem; }
            .prompt-footer { flex-direction: column; align-items: stretch; }
            .btn { justify-content: center; }
            .preview-grid { grid-template-columns: 1fr; }
        }
    </style>
</head>
<body>
    <header class="header">
        <div class="header-left">
            <div class="header-icon">🖼</div>
            <h1>E-Paper Studio</h1>
        </div>
        <div class="header-right">
            <div class="rate-badge">
                <span class="rate-dot {{if le .Remaining 2}}{{if eq .Remaining 0}}danger{{else}}warning{{end}}{{end}}"></span>
                <span>{{.Remaining}}/{{.Limit}} remaining</span>
            </div>
            <a href="/logout" class="logout-link">Sign Out</a>
        </div>
    </header>

    <main class="main">
        {{if .Success}}<div class="alert success">✓ {{.Success}}</div>{{end}}
        {{if .Error}}<div class="alert error">✕ {{.Error}}</div>{{end}}

        <div class="card">
            <div class="card-title">Generate Image</div>
            <form method="POST" action="/prompt" id="promptForm">
                <textarea class="prompt-area" name="prompt" placeholder="Describe the image you want to generate for your e-paper display..." required>{{.Prompt}}</textarea>
                <div class="orientation-row">
                    <label class="orientation-label">Orientation</label>
                    <div class="orientation-toggle">
                        <input type="radio" id="landscape" name="orientation" value="landscape" {{if ne .Orientation "portrait"}}checked{{end}}>
                        <label for="landscape" class="toggle-option">
                            <svg width="18" height="14" viewBox="0 0 18 14" fill="none" xmlns="http://www.w3.org/2000/svg"><rect x="0.5" y="0.5" width="17" height="13" rx="1.5" stroke="currentColor"/></svg>
                            Landscape
                        </label>
                        <input type="radio" id="portrait" name="orientation" value="portrait" {{if eq .Orientation "portrait"}}checked{{end}}>
                        <label for="portrait" class="toggle-option">
                            <svg width="14" height="18" viewBox="0 0 14 18" fill="none" xmlns="http://www.w3.org/2000/svg"><rect x="0.5" y="0.5" width="13" height="17" rx="1.5" stroke="currentColor"/></svg>
                            Portrait
                        </label>
                    </div>
                </div>
                <div class="prompt-footer">
                    <span class="provider-info">Provider: <span class="provider-name">{{.ProviderName}}</span></span>
                    <button type="submit" class="btn" id="generateBtn" {{if eq .Remaining 0}}disabled{{end}}>
                        <span class="btn-text">✨ Generate</span>
                        <span class="spinner"></span>
                    </button>
                </div>
            </form>
        </div>

        <div class="card">
            <div class="card-title">OTA Firmware Update</div>
            <form method="POST" action="/firmware/upload" enctype="multipart/form-data">
                <div class="form-group" style="margin-bottom: 1rem;">
                    <input type="file" name="firmware" accept=".bin" required class="prompt-area" style="min-height: auto; padding: 0.75rem;" />
                </div>
                <div class="prompt-footer">
                    <span class="provider-info">{{if .HasFirmware}}Current firmware: {{.FirmwareUpdatedAt}}{{else}}No firmware uploaded yet{{end}}</span>
                    <button type="submit" class="btn">
                        <span class="btn-text">⬆️ Upload .bin</span>
                    </button>
                </div>
            </form>
            {{if and .HasFirmware .CdnDomain}}
            <div style="margin-top: 1rem; text-align: center; font-size: 0.8rem; color: #a1a1aa; padding-top: 1rem; border-top: 1px solid rgba(255, 255, 255, 0.06);">
                Hosted at: <a href="https://{{.CdnDomain}}/firmware/current.bin" target="_blank" style="color: #8b5cf6; text-decoration: none;">https://{{.CdnDomain}}/firmware/current.bin</a>
            </div>
            {{end}}
        </div>

        <div class="card">
            <div class="card-title">Current Display Image</div>
            <div class="preview-section">
                {{if .HasImage}}
                <div class="preview-grid">
                    <div class="preview-item">
                        <div class="preview-label">Original</div>
                        <div class="preview-container">
                            <img src="/image/original?t={{.UpdatedAt}}" alt="Original image" />
                        </div>
                    </div>
                    <div class="preview-item">
                        <div class="preview-label">Dithered (E-Paper)</div>
                        <div class="preview-container">
                            <img src="/image?t={{.UpdatedAt}}" alt="Dithered e-paper image" />
                        </div>
                    </div>
                </div>
                <div class="preview-meta">
                    <span>Updated: {{.UpdatedAt}}</span>
                </div>
                {{if .CdnDomain}}
                <div style="margin-top: 1rem; text-align: center; font-size: 0.8rem; color: #a1a1aa; display: flex; flex-direction: column; gap: 0.25rem;">
                    <span>Hosted at: <a href="https://{{.CdnDomain}}/image/current.bmp" target="_blank" style="color: #8b5cf6; text-decoration: none;">https://{{.CdnDomain}}/image/current.bmp</a></span>
                    <span>Preview original: <a href="https://{{.CdnDomain}}/image/original.jpg" target="_blank" style="color: #8b5cf6; text-decoration: none;">https://{{.CdnDomain}}/image/original.jpg</a> (or .png)</span>
                </div>
                {{end}}
                <div class="palette-dots">
                    <span class="palette-dot" style="background: #000000;" title="Black"></span>
                    <span class="palette-dot" style="background: #ffffff;" title="White"></span>
                    <span class="palette-dot" style="background: #e6e600;" title="Yellow"></span>
                    <span class="palette-dot" style="background: #cc0000;" title="Red"></span>
                    <span class="palette-dot" style="background: #0033cc;" title="Blue"></span>
                    <span class="palette-dot" style="background: #00cc00;" title="Green"></span>
                </div>
                {{else}}
                <div class="no-image">
                    <div class="no-image-icon">🖼</div>
                    <p>No image generated yet. Enter a prompt above to get started.</p>
                </div>
                {{end}}
            </div>
        </div>
    </main>

    <script>
        document.getElementById('promptForm').addEventListener('submit', function() {
            const btn = document.getElementById('generateBtn');
            btn.classList.add('loading');
            btn.disabled = true;
        });
    </script>
</body>
</html>{{end}}`
