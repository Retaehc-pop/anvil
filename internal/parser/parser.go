package parser

import (
	"encoding/json"
	"strings"
)

// Format selects how ansible subprocess output is interpreted.
type Format int

const (
	FormatJSON Format = iota
	FormatYAML // human-readable; raw lines are streamed, no structured events
)

// EventType identifies what happened in a JSON-callback event.
type EventType int

const (
	EventUnknown EventType = iota
	EventPlayStart
	EventTaskStart
	EventRunnerOK
	EventRunnerChanged
	EventRunnerFailed
	EventRunnerSkipped
	EventRunnerUnreachable
	EventStats
	EventRaw // FormatYAML or unrecognised JSON lines
)

// Event is a parsed ansible output event.
type Event struct {
	Type     EventType
	Play     string
	Task     string
	Host     string
	Msg   string
	Raw   string
	Stats *StatsPayload
}

// StatsPayload holds the final play recap counters.
type StatsPayload struct {
	Ok          map[string]int
	Changed     map[string]int
	Failed      map[string]int
	Skipped     map[string]int
	Unreachable map[string]int
}

// Parse converts one line of ansible output into an Event.
// For FormatYAML, every non-empty line becomes an EventRaw event.
func Parse(line string, format Format) Event {
	line = strings.TrimSpace(line)
	if line == "" {
		return Event{Type: EventUnknown}
	}
	if format == FormatYAML {
		return Event{Type: EventRaw, Raw: line}
	}
	return parseJSON(line)
}

func parseJSON(line string) Event {
	var envelope struct {
		Event string          `json:"event"`
		Data  json.RawMessage `json:"event_data"`
	}
	if err := json.Unmarshal([]byte(line), &envelope); err != nil {
		return Event{Type: EventRaw, Raw: line}
	}

	ev := Event{Raw: line}
	switch envelope.Event {
	case "v2_playbook_on_play_start":
		var d struct {
			Play struct{ Name string } `json:"play"`
		}
		json.Unmarshal(envelope.Data, &d) //nolint:errcheck
		ev.Type = EventPlayStart
		ev.Play = d.Play.Name

	case "v2_playbook_on_task_start":
		var d struct {
			Task struct{ Name string } `json:"task"`
		}
		json.Unmarshal(envelope.Data, &d) //nolint:errcheck
		ev.Type = EventTaskStart
		ev.Task = d.Task.Name

	case "v2_runner_on_ok":
		ev.Type = EventRunnerOK
		ev.Host, ev.Msg = extractHostMsg(envelope.Data)

	case "v2_runner_on_changed":
		ev.Type = EventRunnerChanged
		ev.Host, ev.Msg = extractHostMsg(envelope.Data)

	case "v2_runner_on_failed":
		ev.Type = EventRunnerFailed
		ev.Host, ev.Msg = extractHostMsg(envelope.Data)

	case "v2_runner_on_skipped":
		ev.Type = EventRunnerSkipped
		ev.Host, _ = extractHostMsg(envelope.Data)

	case "v2_runner_on_unreachable":
		ev.Type = EventRunnerUnreachable
		ev.Host, ev.Msg = extractHostMsg(envelope.Data)

	case "v2_playbook_on_stats":
		ev.Type = EventStats
		ev.Stats = extractStats(envelope.Data)

	default:
		ev.Type = EventUnknown
	}
	return ev
}

func extractHostMsg(data json.RawMessage) (host, msg string) {
	var d struct {
		Host   string         `json:"host"`
		Result map[string]any `json:"res"`
	}
	if err := json.Unmarshal(data, &d); err != nil {
		return "", ""
	}
	if m, ok := d.Result["msg"]; ok {
		msg, _ = m.(string)
	}
	return d.Host, msg
}

func extractStats(data json.RawMessage) *StatsPayload {
	var d struct {
		Ok          map[string]int `json:"ok"`
		Changed     map[string]int `json:"changed"`
		Failures    map[string]int `json:"failures"`
		Skipped     map[string]int `json:"skipped"`
		Dark        map[string]int `json:"dark"` // unreachable
	}
	if err := json.Unmarshal(data, &d); err != nil {
		return nil
	}
	return &StatsPayload{
		Ok:          d.Ok,
		Changed:     d.Changed,
		Failed:      d.Failures,
		Skipped:     d.Skipped,
		Unreachable: d.Dark,
	}
}
