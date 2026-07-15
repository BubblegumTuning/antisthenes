package agent

import (
	"fmt"
	"strings"

	"github.com/nanami/antisthenes/internal/cron"
)

// RegisterCronTools adds schedule_task, list_tasks, and cancel_task.
// Pass a non-nil scheduler when cron is enabled; nil registers tools that report unavailability.
func RegisterCronTools(r *ToolRegistry, sched *cron.Scheduler) {
	r.Register("schedule_task", func(args map[string]any) (string, error) {
		if sched == nil {
			return "schedule_task: cron scheduler not active (set cron_enabled: true in config)", nil
		}
		id, ok := args["id"].(string)
		if !ok || strings.TrimSpace(id) == "" {
			return "schedule_task: id is required", nil
		}
		schedule, ok := args["schedule"].(string)
		if !ok || strings.TrimSpace(schedule) == "" {
			return "schedule_task: schedule is required (e.g. \"every 5m\")", nil
		}
		command, ok := args["command"].(string)
		if !ok || strings.TrimSpace(command) == "" {
			return "schedule_task: command is required", nil
		}
		id = strings.TrimSpace(id)
		schedule = strings.TrimSpace(schedule)
		command = strings.TrimSpace(command)

		sched.Schedule(id, schedule, command, nil)
		return fmt.Sprintf("schedule_task: scheduled %q (%s) -> %q", id, schedule, command), nil
	})

	r.Register("list_tasks", func(args map[string]any) (string, error) {
		if sched == nil {
			return "list_tasks: cron scheduler not active (set cron_enabled: true in config)", nil
		}
		tasks := sched.List()
		if len(tasks) == 0 {
			return "list_tasks: no scheduled tasks", nil
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("list_tasks: %d task(s)\n", len(tasks)))
		for _, t := range tasks {
			fmt.Fprintf(&b, "- %s schedule=%q command=%q next_run=%s\n",
				t.ID, t.Schedule, t.Command, t.NextRun.Format("2006-01-02 15:04:05"))
		}
		return b.String(), nil
	})

	r.Register("cancel_task", func(args map[string]any) (string, error) {
		if sched == nil {
			return "cancel_task: cron scheduler not active (set cron_enabled: true in config)", nil
		}
		id, ok := args["id"].(string)
		if !ok || strings.TrimSpace(id) == "" {
			return "cancel_task: id is required", nil
		}
		id = strings.TrimSpace(id)
		if !sched.Cancel(id) {
			return "cancel_task: task not found: " + id, nil
		}
		return "cancel_task: cancelled " + id, nil
	})
}
