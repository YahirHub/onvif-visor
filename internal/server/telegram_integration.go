package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"onvif-visor/internal/account"
	"onvif-visor/internal/analytics"
	"onvif-visor/internal/media"
	onvifsvc "onvif-visor/internal/onvif"
	"onvif-visor/internal/store"
	tg "onvif-visor/internal/telegram"
)

const telegramPersonVideoCallbackPrefix = "pv:"

type telegramStatusView struct {
	Configured           bool                   `json:"configured"`
	BotID                int64                  `json:"bot_id,omitempty"`
	BotUsername          string                 `json:"bot_username,omitempty"`
	BotURL               string                 `json:"bot_url,omitempty"`
	NotificationsEnabled bool                   `json:"notifications_enabled"`
	MediaMode            string                 `json:"media_mode"`
	VideoSeconds         int                    `json:"video_seconds"`
	Chats                []account.TelegramChat `json:"chats"`
}

func (a *App) handleTelegramStatus(w http.ResponseWriter, r *http.Request) {
	cfg, ok, err := a.accounts.GetTelegramConfig(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "No se pudo consultar Telegram.")
		return
	}
	view := telegramStatusView{Configured: ok, MediaMode: "photo", VideoSeconds: account.TelegramVideoDefaultSeconds}
	if ok {
		view.BotID = cfg.BotID
		view.BotUsername = cfg.BotUsername
		view.BotURL = telegramBotURL(cfg.BotUsername)
		view.NotificationsEnabled = cfg.NotificationsEnabled
		view.MediaMode = cfg.MediaMode
		view.VideoSeconds = cfg.VideoSeconds
		chats, listErr := a.accounts.ListTelegramChats(r.Context())
		if listErr != nil {
			writeError(w, http.StatusInternalServerError, "No se pudieron consultar los chats vinculados.")
			return
		}
		view.Chats = chats
	}
	writeJSON(w, http.StatusOK, view)
}

func (a *App) handleTelegramSave(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token                string `json:"token"`
		NotificationsEnabled bool   `json:"notifications_enabled"`
		MediaMode            string `json:"media_mode"`
		VideoSeconds         int    `json:"video_seconds"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Configuración de Telegram inválida.")
		return
	}
	req.Token = strings.TrimSpace(req.Token)
	req.MediaMode = strings.ToLower(strings.TrimSpace(req.MediaMode))
	if req.MediaMode == "" {
		req.MediaMode = "photo"
	}
	if req.VideoSeconds == 0 {
		req.VideoSeconds = account.TelegramVideoDefaultSeconds
	}
	if req.MediaMode != "photo" && req.MediaMode != "video" && req.MediaMode != "photo_video" {
		writeError(w, http.StatusBadRequest, "El medio debe ser foto, vídeo o foto + vídeo.")
		return
	}
	if req.VideoSeconds < account.TelegramVideoMinSeconds || req.VideoSeconds > account.TelegramVideoMaxSeconds {
		writeError(w, http.StatusBadRequest, "El fragmento debe durar entre 3 y 60 segundos.")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	current, exists, err := a.accounts.GetTelegramConfig(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "No se pudo consultar la integración.")
		return
	}
	if req.Token == "" {
		if !exists {
			writeError(w, http.StatusBadRequest, "Escribe el token del bot de Telegram.")
			return
		}
		if err := a.accounts.UpdateTelegramSettings(ctx, req.NotificationsEnabled, req.MediaMode, req.VideoSeconds); err != nil {
			writeError(w, http.StatusInternalServerError, "No se pudo guardar la configuración.")
			return
		}
		current.NotificationsEnabled = req.NotificationsEnabled
		current.MediaMode = req.MediaMode
		current.VideoSeconds = req.VideoSeconds
	} else {
		if len(req.Token) > 256 || !strings.Contains(req.Token, ":") {
			writeError(w, http.StatusBadRequest, "El token de Telegram no tiene un formato válido.")
			return
		}
		bot, err := tg.New().GetMe(ctx, req.Token)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Telegram rechazó el token. Comprueba el token de BotFather.")
			return
		}
		ciphertext, err := a.store.EncryptSecret(req.Token)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "No se pudo proteger el token.")
			return
		}
		current = account.TelegramConfig{
			TokenCipher: ciphertext, BotID: bot.ID, BotUsername: bot.Username,
			NotificationsEnabled: req.NotificationsEnabled, MediaMode: req.MediaMode, VideoSeconds: req.VideoSeconds,
		}
		if err := a.accounts.SaveTelegramConfig(ctx, current); err != nil {
			writeError(w, http.StatusInternalServerError, "No se pudo guardar Telegram.")
			return
		}
	}
	a.restartTelegramIntegration()
	a.handleTelegramStatus(w, r)
}

func (a *App) handleTelegramDelete(w http.ResponseWriter, r *http.Request) {
	a.stopTelegramIntegration()
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if err := a.accounts.DeleteTelegramConfig(ctx); err != nil {
		a.startTelegramIntegration()
		writeError(w, http.StatusInternalServerError, "No se pudo eliminar la integración.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleTelegramCode(w http.ResponseWriter, r *http.Request) {
	current, ok := currentSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "La sesión no es válida.")
		return
	}
	cfg, configured, err := a.accounts.GetTelegramConfig(r.Context())
	if err != nil || !configured {
		writeError(w, http.StatusConflict, "Configura primero un bot de Telegram.")
		return
	}
	code, expires, err := a.accounts.CreateTelegramCode(r.Context(), current.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "No se pudo generar el código.")
		return
	}
	botURL := telegramBotURL(cfg.BotUsername)
	deepLink := botURL + "?start=" + url.QueryEscape(code)
	writeJSON(w, http.StatusOK, map[string]any{
		"code": code, "expires_at": expires, "bot_url": botURL, "deep_link": deepLink,
		"commands": []string{"/start " + code, "/login " + code},
	})
}

func telegramBotURL(username string) string {
	username = strings.TrimPrefix(strings.TrimSpace(username), "@")
	if username == "" {
		return ""
	}
	return "https://t.me/" + username
}

func (a *App) startTelegramIntegration() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	cfg, ok, err := a.accounts.GetTelegramConfig(ctx)
	cancel()
	if err != nil || !ok {
		if err != nil {
			log.Printf("integration=telegram action=load error=%v", err)
		}
		return
	}
	token, err := a.store.DecryptSecret(cfg.TokenCipher)
	if err != nil {
		log.Printf("integration=telegram action=decrypt-token error=%v", err)
		return
	}
	a.telegramMu.Lock()
	if a.telegramCancel != nil {
		a.telegramCancel()
	}
	pollCtx, pollCancel := context.WithCancel(a.ctx)
	a.telegramCancel = pollCancel
	a.telegramMu.Unlock()
	go a.telegramPollLoop(pollCtx, token)
}

func (a *App) stopTelegramIntegration() {
	a.telegramMu.Lock()
	cancel := a.telegramCancel
	a.telegramCancel = nil
	a.telegramMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (a *App) restartTelegramIntegration() {
	a.stopTelegramIntegration()
	a.startTelegramIntegration()
}

func (a *App) telegramPollLoop(ctx context.Context, token string) {
	client := tg.New()
	var offset int64
	for ctx.Err() == nil {
		updates, err := client.GetUpdates(ctx, token, offset)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("integration=telegram action=get-updates error=%v", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(3 * time.Second):
			}
			continue
		}
		for _, update := range updates {
			if update.UpdateID >= offset {
				offset = update.UpdateID + 1
			}
			if update.CallbackQuery != nil {
				a.handleTelegramCallback(ctx, client, token, *update.CallbackQuery)
				continue
			}
			if update.Message == nil || strings.TrimSpace(update.Message.Text) == "" {
				continue
			}
			a.handleTelegramMessage(ctx, client, token, *update.Message)
		}
	}
}

func (a *App) handleTelegramCallback(ctx context.Context, client *tg.Client, token string, query tg.CallbackQuery) {
	if query.Message == nil {
		_ = client.AnswerCallbackQuery(ctx, token, query.ID, "Acción no disponible.")
		return
	}
	chatID := query.Message.Chat.ID
	linked, err := a.accounts.TelegramChatLinked(ctx, chatID)
	if err != nil || !linked {
		_ = client.AnswerCallbackQuery(ctx, token, query.ID, "Chat no vinculado.")
		return
	}
	if !strings.HasPrefix(query.Data, telegramPersonVideoCallbackPrefix) {
		_ = client.AnswerCallbackQuery(ctx, token, query.ID, "Acción no reconocida.")
		return
	}
	actionID := strings.TrimPrefix(query.Data, telegramPersonVideoCallbackPrefix)
	action, ok, err := a.accounts.GetTelegramMediaAction(ctx, actionID, chatID)
	if err != nil || !ok {
		_ = client.AnswerCallbackQuery(ctx, token, query.ID, "El video ya no está disponible.")
		return
	}
	_ = client.AnswerCallbackQuery(ctx, token, query.ID, "Preparando video…")
	go a.sendTelegramPersonVideoAction(token, action)
}

func (a *App) sendTelegramPersonVideoAction(token string, action account.TelegramMediaAction) {
	client := tg.New()
	select {
	case a.telegramNotifySem <- struct{}{}:
		defer func() { <-a.telegramNotifySem }()
	default:
		ctx, cancel := context.WithTimeout(a.ctx, 5*time.Second)
		defer cancel()
		_ = client.SendMessage(ctx, token, action.ChatID, "Ya se está generando otro medio. Intenta de nuevo en unos segundos.")
		return
	}

	ctx, cancel := context.WithTimeout(a.ctx, time.Duration(account.TelegramVideoMaxSeconds)*time.Second+35*time.Second)
	defer cancel()
	cfg, ok, err := a.accounts.GetTelegramConfig(ctx)
	if err != nil || !ok {
		return
	}
	maxSeconds := cfg.VideoSeconds
	if maxSeconds < account.TelegramVideoMinSeconds || maxSeconds > account.TelegramVideoMaxSeconds {
		maxSeconds = account.TelegramVideoDefaultSeconds
	}
	path, clipDuration, err := a.telegramPersonRecordingClip(ctx, action.CameraID, action.RecordingName, action.OffsetSecond, maxSeconds)
	if err != nil {
		_ = client.SendMessage(ctx, token, action.ChatID, "No se pudo generar el video de esa detección. Puede que la grabación ya no esté disponible.")
		return
	}
	defer os.Remove(path)
	name := action.CameraID
	if cam, getErr := a.store.Get(action.CameraID); getErr == nil {
		name = cam.Name
	}
	caption := "🎥 Persona · " + name + " · " + strconv.Itoa(int(math.Ceil(clipDuration.Seconds()))) + " s"
	if err := client.SendVideoFile(ctx, token, action.ChatID, path, caption); err != nil {
		_ = client.SendMessage(ctx, token, action.ChatID, "El video se generó, pero Telegram no pudo recibirlo.")
	}
}

func (a *App) personSegmentNear(ctx context.Context, cameraID, recordingName string, offset float64) (analytics.ObjectSegment, bool, error) {
	if a.analytics == nil {
		return analytics.ObjectSegment{}, false, nil
	}
	segments, err := a.analytics.ObjectSegmentsForRecording(ctx, cameraID, recordingName)
	if err != nil {
		return analytics.ObjectSegment{}, false, err
	}
	bestDistance := math.MaxFloat64
	var best analytics.ObjectSegment
	found := false
	for _, segment := range segments {
		if segment.ClassName != "person" {
			continue
		}
		distance := 0.0
		if offset < segment.StartSecond {
			distance = segment.StartSecond - offset
		} else if offset > segment.EndSecond {
			distance = offset - segment.EndSecond
		}
		if distance < bestDistance {
			bestDistance = distance
			best = segment
			found = true
		}
	}
	if !found || bestDistance > 4 {
		return analytics.ObjectSegment{}, false, nil
	}
	return best, true, nil
}

func (a *App) telegramPersonRecordingClip(ctx context.Context, cameraID, recordingName string, offset float64, maxSeconds int) (string, time.Duration, error) {
	if maxSeconds < account.TelegramVideoMinSeconds || maxSeconds > account.TelegramVideoMaxSeconds {
		maxSeconds = account.TelegramVideoDefaultSeconds
	}
	waitCtx, cancel := context.WithTimeout(ctx, time.Duration(maxSeconds+8)*time.Second)
	defer cancel()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		metadata, metaErr := a.media.RecordingMetadata(waitCtx, cameraID, recordingName)
		if metaErr != nil {
			return "", 0, metaErr
		}
		segment, found, segmentErr := a.personSegmentNear(waitCtx, cameraID, recordingName, offset)
		if segmentErr != nil {
			return "", 0, segmentErr
		}
		if found {
			start := math.Max(0, segment.StartSecond)
			end := math.Max(start+1, segment.EndSecond)
			durationSeconds := end - start
			settled := !metadata.Active || metadata.DurationSecond-end >= 3.5
			if durationSeconds >= float64(maxSeconds) {
				durationSeconds = float64(maxSeconds)
				settled = true
			}
			if settled {
				duration := time.Duration(math.Ceil(durationSeconds*1000)) * time.Millisecond
				path, clipErr := a.media.RecordingClip(waitCtx, cameraID, recordingName, start, duration)
				return path, duration, clipErr
			}
		}
		select {
		case <-waitCtx.Done():
			fallback := time.Duration(maxSeconds) * time.Second
			start := math.Max(0, offset-1)
			path, clipErr := a.media.RecordingClip(ctx, cameraID, recordingName, start, fallback)
			return path, fallback, clipErr
		case <-ticker.C:
		}
	}
}

type telegramCommand struct {
	Name string
	Arg  string
}

func parseTelegramCommand(text string) telegramCommand {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) == 0 || !strings.HasPrefix(fields[0], "/") {
		return telegramCommand{}
	}
	name := strings.ToLower(strings.SplitN(fields[0], "@", 2)[0])
	arg := ""
	if len(fields) > 1 {
		arg = strings.TrimSpace(strings.Join(fields[1:], " "))
	}
	return telegramCommand{Name: name, Arg: arg}
}

func (a *App) handleTelegramMessage(ctx context.Context, client *tg.Client, token string, message tg.Message) {
	command := parseTelegramCommand(message.Text)
	if command.Name == "" {
		return
	}
	linked, err := a.accounts.TelegramChatLinked(ctx, message.Chat.ID)
	if err != nil {
		return
	}

	switch command.Name {
	case "/start", "/login":
		if command.Arg == "" {
			if linked {
				_ = client.SendMessage(ctx, token, message.Chat.ID, "Este chat ya está vinculado. Usa /camaras, /foto <cámara> o /video <cámara> [segundos].")
			} else {
				_ = client.SendMessage(ctx, token, message.Chat.ID, "Genera un código de 4 dígitos en ONVIF Visor y envía /login CODIGO. El código dura 10 minutos.")
			}
			return
		}
		if !validFourDigitCode(command.Arg) || !a.allowTelegramCodeAttempt(message.Chat.ID) {
			_ = client.SendMessage(ctx, token, message.Chat.ID, "Código inválido o demasiados intentos. Genera uno nuevo desde ONVIF Visor.")
			return
		}
		userID, ok, err := a.accounts.ConsumeTelegramCode(ctx, command.Arg)
		if err != nil || !ok {
			a.recordTelegramCodeFailure(message.Chat.ID)
			_ = client.SendMessage(ctx, token, message.Chat.ID, "Código inválido o vencido.")
			return
		}
		a.clearTelegramCodeFailures(message.Chat.ID)
		if err := a.accounts.LinkTelegramChat(ctx, userID, message.Chat.ID, tg.ChatDisplayName(message.Chat)); err != nil {
			_ = client.SendMessage(ctx, token, message.Chat.ID, "No se pudo vincular el chat.")
			return
		}
		_ = client.SendMessage(ctx, token, message.Chat.ID, "Chat vinculado correctamente con ONVIF Visor. Usa /camaras, /foto <cámara> o /video <cámara> [segundos].")
	case "/camaras":
		if !linked {
			a.telegramLoginRequired(ctx, client, token, message.Chat.ID)
			return
		}
		cams := a.store.List()
		if len(cams) == 0 {
			_ = client.SendMessage(ctx, token, message.Chat.ID, "No hay cámaras configuradas.")
			return
		}
		lines := []string{"Cámaras disponibles:"}
		for _, cam := range cams {
			lines = append(lines, "• "+cam.Name)
		}
		_ = client.SendMessage(ctx, token, message.Chat.ID, strings.Join(lines, "\n"))
	case "/foto", "/photo":
		if !linked {
			a.telegramLoginRequired(ctx, client, token, message.Chat.ID)
			return
		}
		a.sendTelegramCameraPhoto(ctx, client, token, message.Chat.ID, command.Arg)
	case "/video":
		if !linked {
			a.telegramLoginRequired(ctx, client, token, message.Chat.ID)
			return
		}
		// Un clip largo no debe bloquear el long polling de Telegram.
		go a.sendTelegramCameraVideo(ctx, client, token, message.Chat.ID, command.Arg)
	}
}

func (a *App) telegramLoginRequired(ctx context.Context, client *tg.Client, token string, chatID int64) {
	_ = client.SendMessage(ctx, token, chatID, "Este chat no está vinculado. Genera un código en ONVIF Visor y usa /login CODIGO.")
}

func validFourDigitCode(code string) bool {
	if len(code) != 4 {
		return false
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func (a *App) allowTelegramCodeAttempt(chatID int64) bool {
	now := time.Now()
	cutoff := now.Add(-10 * time.Minute)
	a.telegramAuthMu.Lock()
	defer a.telegramAuthMu.Unlock()
	attempts := a.telegramAuthFailures[chatID][:0]
	for _, attempt := range a.telegramAuthFailures[chatID] {
		if attempt.After(cutoff) {
			attempts = append(attempts, attempt)
		}
	}
	a.telegramAuthFailures[chatID] = attempts
	return len(attempts) < 5
}

func (a *App) recordTelegramCodeFailure(chatID int64) {
	a.telegramAuthMu.Lock()
	a.telegramAuthFailures[chatID] = append(a.telegramAuthFailures[chatID], time.Now())
	a.telegramAuthMu.Unlock()
}

func (a *App) clearTelegramCodeFailures(chatID int64) {
	a.telegramAuthMu.Lock()
	delete(a.telegramAuthFailures, chatID)
	a.telegramAuthMu.Unlock()
}

func (a *App) resolveTelegramCamera(query string) (store.Camera, error) {
	query = strings.TrimSpace(query)
	cameras := a.store.List()
	if query == "" && len(cameras) == 1 {
		return a.store.Get(cameras[0].ID)
	}
	for _, cam := range cameras {
		if strings.EqualFold(cam.ID, query) || strings.EqualFold(cam.Name, query) {
			return a.store.Get(cam.ID)
		}
	}
	return store.Camera{}, store.ErrNotFound
}

func (a *App) sendTelegramCameraPhoto(ctx context.Context, client *tg.Client, token string, chatID int64, cameraQuery string) {
	cam, err := a.resolveTelegramCamera(cameraQuery)
	if err != nil {
		_ = client.SendMessage(ctx, token, chatID, "Cámara no encontrada. Usa /camaras para ver los nombres disponibles.")
		return
	}
	streamURI, err := onvifsvc.CredentialedRTSP(cam.RTSPURI, cam.Username, cam.Password)
	if err != nil {
		_ = client.SendMessage(ctx, token, chatID, "No se pudo abrir la cámara.")
		return
	}
	mediaCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	image, err := a.media.Snapshot(mediaCtx, streamURI)
	if err != nil {
		_ = client.SendMessage(ctx, token, chatID, "No se pudo obtener la foto en este momento.")
		return
	}
	_ = client.SendPhoto(mediaCtx, token, chatID, image, "📷 "+cam.Name)
}

type telegramVideoRequest struct {
	CameraQuery string
	Seconds     int
}

func parseTelegramVideoRequest(arg string, defaultSeconds int) (telegramVideoRequest, error) {
	arg = strings.TrimSpace(arg)
	if defaultSeconds < account.TelegramVideoMinSeconds || defaultSeconds > account.TelegramVideoMaxSeconds {
		defaultSeconds = account.TelegramVideoDefaultSeconds
	}
	request := telegramVideoRequest{CameraQuery: arg, Seconds: defaultSeconds}
	fields := strings.Fields(arg)
	if len(fields) < 2 {
		return request, nil
	}
	seconds, err := strconv.Atoi(fields[len(fields)-1])
	if err != nil {
		return request, nil
	}
	if seconds < account.TelegramVideoMinSeconds || seconds > account.TelegramVideoMaxSeconds {
		return telegramVideoRequest{}, fmt.Errorf("la duración debe estar entre %d y %d segundos", account.TelegramVideoMinSeconds, account.TelegramVideoMaxSeconds)
	}
	cameraQuery := strings.TrimSpace(strings.Join(fields[:len(fields)-1], " "))
	if cameraQuery == "" {
		return telegramVideoRequest{}, errors.New("indica el nombre de la cámara")
	}
	return telegramVideoRequest{CameraQuery: cameraQuery, Seconds: seconds}, nil
}

func (a *App) sendTelegramCameraVideo(ctx context.Context, client *tg.Client, token string, chatID int64, commandArg string) {
	select {
	case a.telegramNotifySem <- struct{}{}:
		defer func() { <-a.telegramNotifySem }()
	default:
		_ = client.SendMessage(ctx, token, chatID, "Ya se está generando otro medio. Intenta de nuevo en unos segundos.")
		return
	}

	cfg, ok, err := a.accounts.GetTelegramConfig(ctx)
	if err != nil || !ok {
		return
	}
	seconds := cfg.VideoSeconds
	if seconds < account.TelegramVideoMinSeconds || seconds > account.TelegramVideoMaxSeconds {
		seconds = account.TelegramVideoDefaultSeconds
	}

	// Primero intenta el argumento completo. Así una cámara llamada "Patio 2"
	// no se confunde con un override de duración.
	cam, resolveErr := a.resolveTelegramCamera(commandArg)
	if resolveErr != nil {
		request, parseErr := parseTelegramVideoRequest(commandArg, seconds)
		if parseErr != nil {
			_ = client.SendMessage(ctx, token, chatID, "No se pudo preparar el vídeo: "+parseErr.Error()+".")
			return
		}
		seconds = request.Seconds
		cam, resolveErr = a.resolveTelegramCamera(request.CameraQuery)
	}
	if resolveErr != nil {
		_ = client.SendMessage(ctx, token, chatID, "Cámara no encontrada. Usa /camaras para ver los nombres disponibles.")
		return
	}

	streamURI, err := onvifsvc.CredentialedRTSP(cam.RTSPURI, cam.Username, cam.Password)
	if err != nil {
		_ = client.SendMessage(ctx, token, chatID, "No se pudo abrir la cámara.")
		return
	}
	_ = client.SendMessage(ctx, token, chatID, "Preparando fragmento de "+strconv.Itoa(seconds)+" s…")
	mediaCtx, cancel := context.WithTimeout(ctx, time.Duration(seconds)*time.Second+35*time.Second)
	defer cancel()
	path, err := a.media.CaptureClip(mediaCtx, streamURI, time.Duration(seconds)*time.Second)
	if err != nil {
		_ = client.SendMessage(ctx, token, chatID, "No se pudo generar el fragmento en este momento.")
		return
	}
	defer os.Remove(path)
	if err := client.SendVideoFile(mediaCtx, token, chatID, path, "🎥 "+cam.Name+" · "+strconv.Itoa(seconds)+" s"); err != nil {
		_ = client.SendMessage(ctx, token, chatID, "El vídeo se generó, pero Telegram no pudo recibirlo.")
	}
}

func personRecordingSnapshotTarget(status media.RecordingStatus, detectedAt time.Time) (string, float64, bool) {
	if status.Path == "" || status.StartedAt.IsZero() || detectedAt.IsZero() {
		return "", 0, false
	}
	if detectedAt.Before(status.StartedAt) {
		return "", 0, false
	}
	offset := detectedAt.Sub(status.StartedAt).Seconds()
	return filepath.Base(status.Path), offset, true
}

func (a *App) telegramPersonPhoto(ctx context.Context, cam store.Camera, status media.RecordingStatus, detectedAt time.Time) ([]byte, error) {
	// Primero intenta el MKV principal que ya se está grabando. Ese archivo usa
	// la misma geometría del stream principal del panel y el offset corresponde
	// al instante exacto en que NanoDet confirmó a la persona.
	if name, offset, ok := personRecordingSnapshotTarget(status, detectedAt); ok {
		recordingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		image, err := a.media.RecordingSnapshot(recordingCtx, cam.ID, name, offset)
		cancel()
		if err == nil && len(image) > 0 {
			return image, nil
		}
	}

	// El MKV activo puede no haber volcado todavía el cluster del instante
	// detectado. En ese caso toma una captura inmediata del RTSP principal, no
	// del substream ligero que usa NanoDet.
	streamURI, err := onvifsvc.CredentialedRTSP(cam.RTSPURI, cam.Username, cam.Password)
	if err != nil {
		return nil, err
	}
	return a.media.Snapshot(ctx, streamURI)
}

func (a *App) sendTelegramLiveVideoToChats(ctx context.Context, client *tg.Client, token string, chats []account.TelegramChat, cam store.Camera, seconds int, caption string) {
	if seconds < account.TelegramVideoMinSeconds || seconds > account.TelegramVideoMaxSeconds {
		seconds = account.TelegramVideoDefaultSeconds
	}
	streamURI, err := onvifsvc.CredentialedRTSP(cam.RTSPURI, cam.Username, cam.Password)
	if err != nil {
		return
	}
	path, err := a.media.CaptureClip(ctx, streamURI, time.Duration(seconds)*time.Second)
	if err != nil {
		return
	}
	defer os.Remove(path)
	for _, chat := range chats {
		if chat.Enabled {
			_ = client.SendVideoFile(ctx, token, chat.ChatID, path, caption)
		}
	}
}
func telegramPersonRecordWindow(cfg account.TelegramConfig) time.Duration {
	seconds := cfg.VideoSeconds
	if seconds < account.TelegramVideoMinSeconds || seconds > account.TelegramVideoMaxSeconds {
		seconds = account.TelegramVideoDefaultSeconds
	}
	return time.Duration(seconds+8) * time.Second
}

func (a *App) ensureTelegramPersonRecording(cam store.Camera, cfg account.TelegramConfig) media.RecordingStatus {
	status := a.media.Status(cam.ID)
	streamURI, err := onvifsvc.CredentialedRTSP(cam.RTSPURI, cam.Username, cam.Password)
	if err != nil {
		return status
	}
	next, err := a.media.Trigger(cam.ID, streamURI, telegramPersonRecordWindow(cfg))
	if err != nil {
		logCameraError(cam.ID, "telegram-person-record", err)
		return status
	}
	return next
}
func (a *App) sendTelegramPersonPhoto(ctx context.Context, client *tg.Client, token string, chat account.TelegramChat, cam store.Camera, recordingName string, offset float64, image []byte, caption string) {
	if !chat.Enabled {
		return
	}
	if recordingName != "" {
		action, err := a.accounts.CreateTelegramMediaAction(ctx, chat.ChatID, cam.ID, recordingName, offset)
		if err == nil {
			_ = client.SendPhotoWithCallbackButton(ctx, token, chat.ChatID, image, caption, "Ver video", telegramPersonVideoCallbackPrefix+action.ID)
			return
		}
	}
	_ = client.SendPhoto(ctx, token, chat.ChatID, image, caption)
}

func (a *App) sendTelegramPersonVideoToChats(ctx context.Context, client *tg.Client, token string, chats []account.TelegramChat, cam store.Camera, recordingName string, offset float64, maxSeconds int, caption string) {
	if maxSeconds < account.TelegramVideoMinSeconds || maxSeconds > account.TelegramVideoMaxSeconds {
		maxSeconds = account.TelegramVideoDefaultSeconds
	}
	var path string
	var duration time.Duration
	var err error
	if recordingName != "" {
		path, duration, err = a.telegramPersonRecordingClip(ctx, cam.ID, recordingName, offset, maxSeconds)
	}
	if err != nil || path == "" {
		streamURI, uriErr := onvifsvc.CredentialedRTSP(cam.RTSPURI, cam.Username, cam.Password)
		if uriErr != nil {
			return
		}
		duration = time.Duration(maxSeconds) * time.Second
		path, err = a.media.CaptureClip(ctx, streamURI, duration)
		if err != nil {
			return
		}
	}
	defer os.Remove(path)
	videoCaption := caption + "\nVideo · " + strconv.Itoa(int(math.Ceil(duration.Seconds()))) + " s"
	for _, chat := range chats {
		if chat.Enabled {
			_ = client.SendVideoFile(ctx, token, chat.ChatID, path, videoCaption)
		}
	}
}

func (a *App) sendTelegramPersonAlert(ctx context.Context, cfg account.TelegramConfig, client *tg.Client, token string, chats []account.TelegramChat, cam store.Camera, recordingName string, offset float64, image []byte, caption string) {
	if cfg.MediaMode != "video" && len(image) > 0 {
		for _, chat := range chats {
			a.sendTelegramPersonPhoto(ctx, client, token, chat, cam, recordingName, offset, image, caption)
		}
	}
	if cfg.MediaMode == "video" || cfg.MediaMode == "photo_video" {
		a.sendTelegramPersonVideoToChats(ctx, client, token, chats, cam, recordingName, offset, cfg.VideoSeconds, caption)
	}
}
func (a *App) notifyTelegramEvent(cam store.Camera, event onvifsvc.Event, status media.RecordingStatus) {
	if !event.Recording {
		return
	}
	select {
	case a.telegramNotifySem <- struct{}{}:
		defer func() { <-a.telegramNotifySem }()
	default:
		return
	}

	now := time.Now()
	a.telegramNotifyMu.Lock()
	if last := a.telegramLastSent[cam.ID]; !last.IsZero() && now.Sub(last) < 20*time.Second {
		a.telegramNotifyMu.Unlock()
		return
	}
	a.telegramLastSent[cam.ID] = now
	a.telegramNotifyMu.Unlock()

	ctx, cancel := context.WithTimeout(a.ctx, time.Duration(account.TelegramVideoMaxSeconds)*time.Second+35*time.Second)
	defer cancel()
	cfg, ok, err := a.accounts.GetTelegramConfig(ctx)
	if err != nil || !ok || !cfg.NotificationsEnabled {
		return
	}
	chats, err := a.accounts.ListTelegramChats(ctx)
	if err != nil || len(chats) == 0 {
		return
	}
	token, err := a.store.DecryptSecret(cfg.TokenCipher)
	if err != nil {
		return
	}
	client := tg.New()
	eventTime := event.Time
	if eventTime.IsZero() {
		eventTime = time.Now().UTC()
	}
	genericCaption := "⚠️ Evento detectado · " + cam.Name + "\n" + eventTime.Local().Format("02/01/2006 15:04:05")
	if a.analytics == nil || !a.analytics.Status().Ready {
		if cfg.MediaMode == "video" {
			a.sendTelegramLiveVideoToChats(ctx, client, token, chats, cam, cfg.VideoSeconds, genericCaption)
		}
		return
	}

	personCtx, personCancel := context.WithTimeout(ctx, 10*time.Second)
	frame, found := a.analytics.WaitForPersonFrame(personCtx, cam.ID, eventTime.Add(-3*time.Second))
	personCancel()
	if !found {
		if cfg.MediaMode == "video" {
			a.sendTelegramLiveVideoToChats(ctx, client, token, chats, cam, cfg.VideoSeconds, genericCaption)
		}
		return
	}

	status = a.ensureTelegramPersonRecording(cam, cfg)
	recordingName, recordingOffset, _ := personRecordingSnapshotTarget(status, frame.DetectedAt)
	caption := "⚠️ Persona detectada · " + cam.Name +
		"\n" + frame.DetectedAt.Local().Format("02/01/2006 15:04:05") +
		fmt.Sprintf("\nConfianza · %.0f%%", frame.Confidence*100)

	var image []byte
	if cfg.MediaMode != "video" {
		image, err = a.telegramPersonPhoto(ctx, cam, status, frame.DetectedAt)
		if err != nil || len(image) == 0 {
			return
		}
	}
	a.sendTelegramPersonAlert(ctx, cfg, client, token, chats, cam, recordingName, recordingOffset, image, caption)
}

func (a *App) notifyTelegramSmartEvent(cam store.Camera, event analytics.SmartEvent) {
	select {
	case a.telegramNotifySem <- struct{}{}:
		defer func() { <-a.telegramNotifySem }()
	default:
		return
	}
	ctx, cancel := context.WithTimeout(a.ctx, time.Duration(account.TelegramVideoMaxSeconds)*time.Second+35*time.Second)
	defer cancel()
	cfg, ok, err := a.accounts.GetTelegramConfig(ctx)
	if err != nil || !ok || !cfg.NotificationsEnabled {
		return
	}
	chats, err := a.accounts.ListTelegramChats(ctx)
	if err != nil || len(chats) == 0 {
		return
	}
	token, err := a.store.DecryptSecret(cfg.TokenCipher)
	if err != nil {
		return
	}
	caption := "⚠️ " + analyticsEventCaption(event) + " · " + cam.Name
	when := event.OccurredAt
	if when.IsZero() {
		when = time.Now().UTC()
	}
	caption += "\n" + when.Local().Format("02/01/2006 15:04:05")
	if len(event.MatchedRuleNames) > 0 {
		caption += "\nRegla: " + strings.Join(event.MatchedRuleNames, ", ")
	}
	client := tg.New()
	if event.ClassName == "person" {
		status := a.ensureTelegramPersonRecording(cam, cfg)
		if status.Path != "" && !status.StartedAt.IsZero() {
			event.RecordingName = filepath.Base(status.Path)
			event.OffsetSecond = math.Max(0, when.Sub(status.StartedAt).Seconds())
		}
		if event.Confidence > 0 {
			caption += fmt.Sprintf("\nConfianza · %.0f%%", event.Confidence*100)
		}
		var image []byte
		if cfg.MediaMode != "video" {
			if event.RecordingName != "" {
				image, _ = a.media.RecordingSnapshot(ctx, cam.ID, event.RecordingName, math.Max(0, event.OffsetSecond))
			}
			if len(image) == 0 {
				streamURI, uriErr := onvifsvc.CredentialedRTSP(cam.RTSPURI, cam.Username, cam.Password)
				if uriErr != nil {
					return
				}
				image, err = a.media.Snapshot(ctx, streamURI)
				if err != nil {
					return
				}
			}
		}
		a.sendTelegramPersonAlert(ctx, cfg, client, token, chats, cam, event.RecordingName, math.Max(0, event.OffsetSecond), image, caption)
		return
	}
	if cfg.MediaMode == "video" {
		a.sendTelegramLiveVideoToChats(ctx, client, token, chats, cam, cfg.VideoSeconds, caption)
		return
	}
	var image []byte
	if event.RecordingName != "" {
		image, _ = a.media.RecordingSnapshot(ctx, cam.ID, event.RecordingName, math.Max(0, event.OffsetSecond))
	}
	if len(image) == 0 {
		streamURI, uriErr := onvifsvc.CredentialedRTSP(cam.RTSPURI, cam.Username, cam.Password)
		if uriErr != nil {
			return
		}
		image, err = a.media.Snapshot(ctx, streamURI)
		if err != nil {
			return
		}
	}
	for _, chat := range chats {
		if chat.Enabled {
			_ = client.SendPhoto(ctx, token, chat.ChatID, image, caption)
		}
	}
}

func analyticsEventCaption(event analytics.SmartEvent) string {
	label := event.ClassName
	for _, class := range analytics.Classes() {
		if class.Name == event.ClassName {
			label = class.Label
			break
		}
	}
	switch event.Type {
	case "zone_enter":
		return label + " entró en " + event.ZoneName
	case "zone_exit":
		return label + " salió de " + event.ZoneName
	case "line_cross":
		if event.Direction != "" {
			return label + " cruzó " + event.LineName + " · " + event.Direction
		}
		return label + " cruzó " + event.LineName
	default:
		return label + " detectado"
	}
}
