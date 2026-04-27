package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var meetingCmd = &cobra.Command{
	Use:   "meeting",
	Short: "Show meetings",
}

var meetingNextCmd = &cobra.Command{
	Use:   "next",
	Short: "Show the next upcoming meeting",
	RunE: func(_ *cobra.Command, _ []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		next, err := c.NextMeeting(context.Background())
		if err != nil {
			return err
		}
		if next == nil {
			fmt.Println("(no upcoming meetings)")
			return nil
		}
		t := time.Unix(next.StartAt, 0).Local()
		dur := time.Until(t).Round(time.Minute)
		fmt.Printf("%s — %s (in %s)\n", t.Format("Mon 15:04"), next.Title, dur)
		if next.Location != "" {
			fmt.Printf("  @ %s\n", next.Location)
		}
		return nil
	},
}

var meetingListFlags struct {
	days int
}

var meetingListCmd = &cobra.Command{
	Use:   "list",
	Short: "List meetings in the next N days (default 7)",
	RunE: func(_ *cobra.Command, _ []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		now := time.Now()
		from := now.Unix()
		to := now.AddDate(0, 0, meetingListFlags.days).Unix()
		list, err := c.ListMeetings(context.Background(), from, to, false)
		if err != nil {
			return err
		}
		if len(list) == 0 {
			fmt.Println("(no meetings in window)")
			return nil
		}
		for _, m := range list {
			t := time.Unix(m.StartAt, 0).Local()
			fmt.Printf("%s  %s\n", t.Format("Mon 02 Jan 15:04"), m.Title)
		}
		return nil
	},
}

func init() {
	meetingListCmd.Flags().IntVarP(&meetingListFlags.days, "days", "d", 7, "Window size in days")
	meetingCmd.AddCommand(meetingNextCmd, meetingListCmd)
	rootCmd.AddCommand(meetingCmd)
}
