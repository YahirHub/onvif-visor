package media

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// mp4RemuxArgs remuxa una grabación a MP4 sin recodificar el vídeo: copiar el
// stream HEVC o H.264 de las cámaras es instantáneo y no pierde calidad. El
// audio solo se transcodifica a AAC cuando no cabe en MP4 (G.711 y similares).
// El MP4 sale fragmentado para poder servirlo en streaming sin conocer antes
// su tamaño final.
func mp4RemuxArgs(path, videoCodec, audioCodec string) []string {
	args := []string{
		"-hide_banner", "-loglevel", "error",
		"-i", path,
		"-map", "0:v:0",
		"-map", "0:a?",
		"-c:v", "copy",
	}
	if videoCodec == "hevc" {
		// hvc1 es la etiqueta que esperan QuickTime y los reproductores de Apple.
		args = append(args, "-tag:v", "hvc1")
	}
	switch audioCodec {
	case "aac", "mp3", "ac3", "":
		args = append(args, "-c:a", "copy")
	default:
		args = append(args, "-c:a", "aac", "-b:a", "64k")
	}
	return append(args,
		"-avoid_negative_ts", "make_zero",
		"-movflags", "+frag_keyframe+empty_moov+default_base_moof",
		"-f", "mp4", "pipe:1",
	)
}

func (m *Manager) probeStreamCodecs(ctx context.Context, path string) (video, audio string) {
	if m.ffprobe == "" {
		return "", ""
	}
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(probeCtx, m.ffprobe,
		"-v", "error",
		"-show_entries", "stream=codec_type,codec_name",
		"-of", "json",
		path,
	)
	output, err := cmd.Output()
	if err != nil {
		return "", ""
	}
	var parsed struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			CodecName string `json:"codec_name"`
		} `json:"streams"`
	}
	if json.Unmarshal(output, &parsed) != nil {
		return "", ""
	}
	for _, stream := range parsed.Streams {
		switch stream.CodecType {
		case "video":
			if video == "" {
				video = stream.CodecName
			}
		case "audio":
			if audio == "" {
				audio = stream.CodecName
			}
		}
	}
	return video, audio
}

// DownloadMP4 sirve la grabación convertida a MP4 al vuelo. El archivo
// original en Matroska no se toca.
func (m *Manager) DownloadMP4(w http.ResponseWriter, r *http.Request, cameraID, name string) error {
	path, err := m.RecordingPath(cameraID, name)
	if err != nil {
		return err
	}
	if !m.acquirePlayback() {
		return ErrTooManyPlayback
	}
	defer m.releasePlayback()

	videoCodec, audioCodec := m.probeStreamCodecs(r.Context(), path)
	cmd := exec.CommandContext(r.Context(), m.ffmpeg, mp4RemuxArgs(path, videoCodec, audioCodec)...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return err
	}

	downloadName := strings.TrimSuffix(name, ".mkv") + ".mp4"
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": downloadName}))
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, copyErr := io.Copy(w, stdout)
	waitErr := cmd.Wait()
	if r.Context().Err() != nil {
		return nil
	}
	if copyErr != nil {
		return copyErr
	}
	return waitErr
}
