package server

import (
	"context"
	"errors"
	"log"
	"sort"
	"time"

	"onvif-visor/internal/media"
)

const (
	retentionPollInterval = time.Minute
	retentionTargetMargin = 5.0
)

type retentionCandidate struct {
	CameraID string
	Name     string
	Started  time.Time
}

func (a *App) startRetentionManager() {
	go func() {
		a.applyRetention()
		ticker := time.NewTicker(retentionPollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-a.ctx.Done():
				return
			case <-ticker.C:
				a.applyRetention()
			}
		}
	}()
}

func (a *App) applyRetention() {
	stats, err := a.media.RecordingStorageStats()
	if err != nil || stats.DiskTotalBytes <= 0 {
		if err != nil {
			log.Printf("action=retention_stats error=%v", err)
		}
		return
	}

	initialUsage := stats.DiskUsedPercent
	eligible := make(map[string]bool)
	target := 100.0
	for _, cam := range a.store.List() {
		if !cam.AutoRetention || cam.RetentionDiskPercent <= 0 {
			continue
		}
		threshold := float64(cam.RetentionDiskPercent)
		if initialUsage+0.0001 < threshold {
			continue
		}
		eligible[cam.ID] = true
		cameraTarget := threshold - retentionTargetMargin
		if cameraTarget < 50 {
			cameraTarget = 50
		}
		if cameraTarget < target {
			target = cameraTarget
		}
	}
	if len(eligible) == 0 {
		return
	}

	candidates := make([]retentionCandidate, 0)
	for cameraID := range eligible {
		files, listErr := a.media.ListRecordings(cameraID)
		if listErr != nil {
			log.Printf("camera=%s action=retention_list error=%v", cameraID, listErr)
			continue
		}
		for _, file := range files {
			if file.Active || a.isProtected(cameraID, file.Name) {
				continue
			}
			candidates = append(candidates, retentionCandidate{
				CameraID: cameraID,
				Name:     file.Name,
				Started:  file.StartedAt,
			})
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Started.Equal(candidates[j].Started) {
			if candidates[i].CameraID == candidates[j].CameraID {
				return candidates[i].Name < candidates[j].Name
			}
			return candidates[i].CameraID < candidates[j].CameraID
		}
		return candidates[i].Started.Before(candidates[j].Started)
	})

	if len(candidates) == 0 {
		log.Printf("action=retention_cleanup usage=%.1f target=%.1f result=no_candidates", initialUsage, target)
		return
	}

	ctx, cancel := context.WithTimeout(a.ctx, 45*time.Second)
	defer cancel()
	deleted := 0
	for _, candidate := range candidates {
		if ctx.Err() != nil {
			break
		}
		current, statsErr := a.media.RecordingStorageStats()
		if statsErr != nil {
			log.Printf("action=retention_stats error=%v", statsErr)
			break
		}
		if current.DiskUsedPercent <= target {
			stats = current
			break
		}
		if err := a.deleteRecordingArtifacts(ctx, candidate.CameraID, candidate.Name); err != nil {
			if errors.Is(err, media.ErrRecordingActive) || errors.Is(err, media.ErrRecordingNotFound) {
				continue
			}
			log.Printf("camera=%s action=retention_delete name=%s error=%v", candidate.CameraID, candidate.Name, err)
			continue
		}
		deleted++
		stats = current
	}
	finalStats, statsErr := a.media.RecordingStorageStats()
	if statsErr == nil {
		stats = finalStats
	}
	log.Printf(
		"action=retention_cleanup usage_before=%.1f usage_after=%.1f target=%.1f deleted=%d",
		initialUsage, stats.DiskUsedPercent, target, deleted,
	)
}
