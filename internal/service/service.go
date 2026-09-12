package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	Name        = "onvif-visor"
	DisplayName = "ONVIF Visor"
)

type Action string

const (
	ActionStart   Action = "start"
	ActionStop    Action = "stop"
	ActionRestart Action = "restart"
	ActionStatus  Action = "status"
)

type InstallOptions struct {
	SourceDir   string
	Listen      string
	FFmpeg      string
	ONNXRuntime string
	Prepare     func(RuntimeConfig) error
}

type Result struct {
	Manager       string
	InstallDir    string
	Binary        string
	CLI           string
	DataDir       string
	RecordingsDir string
	ModelsDir     string
}

type RuntimeConfig struct {
	Listen        string
	FFmpeg        string
	ONNXRuntime   string
	DataDir       string
	RecordingsDir string
	ModelsDir     string
}

type installMetadata struct {
	Schema      int       `json:"schema"`
	Manager     string    `json:"manager"`
	Listen      string    `json:"listen"`
	FFmpeg      string    `json:"ffmpeg"`
	ONNXRuntime string    `json:"onnxruntime,omitempty"`
	InstalledAt time.Time `json:"installed_at"`
}

type paths struct {
	InstallDir    string
	Binary        string
	DataDir       string
	RecordingsDir string
	ModelsDir     string
	Metadata      string
	LogDir        string
}

type manager interface {
	Name() string
	StopIfInstalled() error
	Install(paths) error
	Remove(paths) error
	Control(Action) (string, error)
}

func Install(opts InstallOptions) (Result, error) {
	p := platformPaths()
	mgr, err := detectManager()
	if err != nil {
		return Result{}, err
	}
	opts.Listen = strings.TrimSpace(opts.Listen)
	if opts.Listen == "" {
		opts.Listen = "0.0.0.0:8081"
	}
	opts.FFmpeg = strings.TrimSpace(opts.FFmpeg)
	if opts.FFmpeg == "" {
		opts.FFmpeg = "ffmpeg"
	}
	opts.ONNXRuntime = strings.TrimSpace(opts.ONNXRuntime)
	if err := mgr.StopIfInstalled(); err != nil {
		return Result{}, fmt.Errorf("stop existing %s service: %w", mgr.Name(), err)
	}
	if err := preparePlatformDirs(p); err != nil {
		return Result{}, err
	}
	if err := installFiles(p, opts.SourceDir); err != nil {
		return Result{}, err
	}
	if opts.Prepare != nil {
		if err := opts.Prepare(RuntimeConfig{Listen: opts.Listen, FFmpeg: opts.FFmpeg, ONNXRuntime: opts.ONNXRuntime, DataDir: p.DataDir, RecordingsDir: p.RecordingsDir, ModelsDir: p.ModelsDir}); err != nil {
			return Result{}, fmt.Errorf("prepare installed data: %w", err)
		}
	}
	meta := installMetadata{
		Schema: 1, Manager: mgr.Name(), Listen: opts.Listen, FFmpeg: opts.FFmpeg, ONNXRuntime: opts.ONNXRuntime,
		InstalledAt: time.Now().UTC(),
	}
	if err := writeJSONAtomic(p.Metadata, meta, 0o644); err != nil {
		return Result{}, fmt.Errorf("write installation metadata: %w", err)
	}
	if err := installCLIEntry(p); err != nil {
		return Result{}, fmt.Errorf("install CLI entry: %w", err)
	}
	if err := mgr.Install(p); err != nil {
		_ = removeCLIEntry(p)
		return Result{}, fmt.Errorf("install %s service: %w", mgr.Name(), err)
	}
	return resultFor(p, mgr.Name()), nil
}

func Remove() (Result, error) {
	p := platformPaths()
	mgr, err := installedManager(p)
	if err != nil {
		return Result{}, err
	}
	result := resultFor(p, mgr.Name())
	if err := mgr.StopIfInstalled(); err != nil {
		return Result{}, fmt.Errorf("stop %s service before removal: %w", mgr.Name(), err)
	}
	if err := mgr.Remove(p); err != nil {
		return Result{}, fmt.Errorf("remove %s service: %w", mgr.Name(), err)
	}
	if err := removeCLIEntry(p); err != nil {
		return Result{}, fmt.Errorf("remove CLI entry: %w", err)
	}
	for _, file := range []string{p.Binary, p.Metadata} {
		if err := os.Remove(file); err != nil && !errors.Is(err, os.ErrNotExist) {
			return Result{}, fmt.Errorf("remove installed file %s: %w", file, err)
		}
	}
	// data/ y recordings/ se conservan intencionalmente.
	return result, nil
}

func Control(action Action) (string, error) {
	switch action {
	case ActionStart, ActionStop, ActionRestart, ActionStatus:
	default:
		return "", fmt.Errorf("unsupported service action %q", action)
	}
	mgr, err := installedManager(platformPaths())
	if err != nil {
		return "", err
	}
	return mgr.Control(action)
}

func Runtime() (RuntimeConfig, error) {
	p := platformPaths()
	meta, err := readMetadata(p)
	if err != nil {
		return RuntimeConfig{}, err
	}
	listen := strings.TrimSpace(meta.Listen)
	if listen == "" {
		listen = "0.0.0.0:8081"
	}
	ffmpeg := strings.TrimSpace(meta.FFmpeg)
	if ffmpeg == "" {
		ffmpeg = "ffmpeg"
	}
	return RuntimeConfig{
		Listen: listen, FFmpeg: ffmpeg, ONNXRuntime: strings.TrimSpace(meta.ONNXRuntime), DataDir: p.DataDir, RecordingsDir: p.RecordingsDir, ModelsDir: p.ModelsDir,
	}, nil
}

func InstallDir() string { return platformPaths().InstallDir }

func installedManager(p paths) (manager, error) {
	meta, err := readMetadata(p)
	if err == nil && meta.Manager != "" {
		if mgr, managerErr := managerByName(meta.Manager); managerErr == nil {
			return mgr, nil
		}
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return detectManager()
}

func readMetadata(p paths) (installMetadata, error) {
	raw, err := os.ReadFile(p.Metadata)
	if err != nil {
		return installMetadata{}, err
	}
	var meta installMetadata
	if err := json.Unmarshal(raw, &meta); err != nil {
		return installMetadata{}, fmt.Errorf("decode installation metadata: %w", err)
	}
	return meta, nil
}

func resultFor(p paths, managerName string) Result {
	return Result{
		Manager: managerName, InstallDir: p.InstallDir, Binary: p.Binary, CLI: cliEntryPath(p),
		DataDir: p.DataDir, RecordingsDir: p.RecordingsDir, ModelsDir: p.ModelsDir,
	}
}

func installFiles(p paths, sourceDir string) error {
	if err := os.MkdirAll(p.InstallDir, 0o755); err != nil {
		return fmt.Errorf("create installation directory: %w", err)
	}
	sourceBinary, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve current executable: %w", err)
	}
	if err := copyExecutable(sourceBinary, p.Binary); err != nil {
		return err
	}

	if strings.TrimSpace(sourceDir) == "" {
		if cwd, cwdErr := os.Getwd(); cwdErr == nil {
			sourceDir = cwd
		}
	}
	if sourceDir == "" {
		return nil
	}
	if err := copyTreeIfDestinationEmpty(filepath.Join(sourceDir, "data"), p.DataDir, 0o700, 0o600); err != nil {
		return fmt.Errorf("migrate data directory: %w", err)
	}
	if err := copyTreeIfDestinationEmpty(filepath.Join(sourceDir, "recordings"), p.RecordingsDir, 0o700, 0o600); err != nil {
		return fmt.Errorf("migrate recordings directory: %w", err)
	}
	return nil
}

func copyExecutable(source, destination string) error {
	sourceAbs, _ := filepath.Abs(source)
	destinationAbs, _ := filepath.Abs(destination)
	if filepath.Clean(sourceAbs) == filepath.Clean(destinationAbs) {
		return nil
	}
	in, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open current executable: %w", err)
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("create binary directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(destination), ".onvif-visor-*")
	if err != nil {
		return fmt.Errorf("create binary temporary file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := io.Copy(tmp, in); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("copy executable: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync executable: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close executable: %w", err)
	}
	if err := applyFileMode(tmpPath, 0o755); err != nil {
		return fmt.Errorf("set executable permissions: %w", err)
	}
	if err := replaceFile(tmpPath, destination); err != nil {
		return fmt.Errorf("replace installed executable: %w", err)
	}
	return nil
}

func copyTreeIfDestinationEmpty(source, destination string, dirMode, fileMode fs.FileMode) error {
	sourceInfo, err := os.Stat(source)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !sourceInfo.IsDir() {
		return fmt.Errorf("source is not a directory: %s", source)
	}
	if err := os.MkdirAll(destination, dirMode); err != nil {
		return err
	}
	empty, err := directoryEmpty(destination)
	if err != nil {
		return err
	}
	if !empty {
		return nil
	}
	return copyTreeSecure(source, destination, dirMode, fileMode)
}

func directoryEmpty(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}

func copyTreeSecure(source, destination string, dirMode, fileMode fs.FileMode) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, rel)
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing symbolic link in migration source: %s", path)
		}
		if entry.IsDir() {
			if err := os.MkdirAll(target, dirMode); err != nil {
				return err
			}
			return os.Chmod(target, dirMode)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		return copyRegularFile(path, target, fileMode)
	})
}

func copyRegularFile(source, destination string, mode fs.FileMode) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	if copyErr == nil {
		copyErr = out.Sync()
	}
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Chmod(destination, mode)
}

func writeJSONAtomic(path string, value any, mode fs.FileMode) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".install-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := applyFileMode(tmpPath, mode); err != nil {
		return err
	}
	return replaceFile(tmpPath, path)
}
