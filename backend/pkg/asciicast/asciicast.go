package asciicast

import (
	"encoding/json"
	"time"
)

// Recording represents an asciicast v2 recording
type Recording struct {
	Version   int            `json:"version"`
	Width     int            `json:"width"`
	Height    int            `json:"height"`
	Timestamp int64          `json:"timestamp"`
	Title     string         `json:"title,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	Events    []Event        `json:"events,omitempty"`
	Duration  time.Duration  `json:"-"`
	startTime time.Time      `json:"-"`
}

// Event represents a single terminal event [time, type, data]
type Event struct {
	Time float64 `json:"time"`
	Type string  `json:"type"` // "o" = output, "i" = input
	Data string  `json:"data"`
}

// MarshalJSON for custom Event serialization [time, type, data]
func (e Event) MarshalJSON() ([]byte, error) {
	return json.Marshal([]interface{}{e.Time, e.Type, e.Data})
}

func (e *Event) UnmarshalJSON(data []byte) error {
	var raw []interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if len(raw) >= 3 {
		if t, ok := raw[0].(float64); ok {
			e.Time = t
		}
		if t, ok := raw[1].(string); ok {
			e.Type = t
		}
		if d, ok := raw[2].(string); ok {
			e.Data = d
		}
	}
	return nil
}

// NewRecorder creates a new asciicast v2 recorder
func NewRecorder(width, height int, title string) *Recorder {
	now := time.Now()
	return &Recorder{
		recording: &Recording{
			Version:   2,
			Width:     width,
			Height:    height,
			Timestamp: now.Unix(),
			Title:     title,
			Env: map[string]string{
				"SHELL": "/bin/bash",
				"TERM":  "xterm-256color",
			},
			startTime: now,
		},
	}
}

// Recorder records terminal I/O
type Recorder struct {
	recording *Recording
	cols      int
	rows      int
}

func (r *Recorder) RecordOutput(data []byte) {
	elapsed := time.Since(r.recording.startTime).Seconds()
	r.recording.Events = append(r.recording.Events, Event{
		Time: elapsed,
		Type: "o",
		Data: string(data),
	})
}

func (r *Recorder) RecordInput(data []byte) {
	elapsed := time.Since(r.recording.startTime).Seconds()
	r.recording.Events = append(r.recording.Events, Event{
		Time: elapsed,
		Type: "i",
		Data: string(data),
	})
}

func (r *Recorder) Resize(cols, rows uint16) {
	r.cols = int(cols)
	r.rows = int(rows)
}

func (r *Recorder) Stop() *Recording {
	r.recording.Duration = time.Since(r.recording.startTime)
	if r.cols > 0 {
		r.recording.Width = r.cols
	}
	if r.rows > 0 {
		r.recording.Height = r.rows
	}
	return r.recording
}
