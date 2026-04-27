package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// timerCmd is the root sub-command for time-tracking operations.
var timerCmd = &cobra.Command{
	Use:   "timer",
	Short: "Time-tracking on todos",
}

var timerStartCmd = &cobra.Command{
	Use:   "start <id-suffix>",
	Short: "Start a timer on a todo",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		id, err := resolveID(c, args[0])
		if err != nil {
			return err
		}
		s, err := c.StartTimer(context.Background(), id)
		if err != nil {
			return err
		}
		fmt.Printf("started %s on %s at %s\n", shortID(s.TodoID), s.TodoID, time.Unix(s.StartedAt, 0).Format(time.RFC3339))
		return nil
	},
}

var timerStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the active timer",
	RunE: func(_ *cobra.Command, _ []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		s, err := c.StopTimer(context.Background())
		if err != nil {
			return err
		}
		dur := time.Duration(0)
		if s.EndedAt != nil {
			dur = time.Duration(*s.EndedAt-s.StartedAt) * time.Second
		}
		fmt.Printf("stopped %s after %s\n", shortID(s.TodoID), dur)
		return nil
	},
}

var timerStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the active timer (if any)",
	RunE: func(_ *cobra.Command, _ []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		s, err := c.ActiveTimer(context.Background())
		if err != nil {
			return err
		}
		if s == nil {
			fmt.Println("(no active timer)")
			return nil
		}
		started := time.Unix(s.StartedAt, 0)
		dur := time.Since(started).Round(time.Second)
		fmt.Printf("active: %s on %s (running for %s)\n", shortID(s.TodoID), s.TodoID, dur)
		return nil
	},
}

func init() {
	timerCmd.AddCommand(timerStartCmd, timerStopCmd, timerStatusCmd)
	rootCmd.AddCommand(timerCmd)
}

// shortID returns the last 6 characters of id, or id itself if shorter.
func shortID(id string) string {
	if len(id) <= 6 {
		return id
	}
	return id[len(id)-6:]
}
