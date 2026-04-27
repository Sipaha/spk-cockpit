package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/spk/spk-cockpit/internal/paths"
	"github.com/spk/spk-cockpit/internal/standup"
)

var standupFlags struct {
	date string
}

var standupCmd = &cobra.Command{
	Use:   "standup",
	Short: "Print today's standup as markdown (yesterday/today/blockers)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		p, err := paths.New()
		if err != nil {
			return err
		}
		c := NewClient(p.SocketFile)
		report, err := c.Standup(cmd.Context(), standupFlags.date)
		if err != nil {
			return err
		}
		fmt.Print(standup.Markdown(report))
		return nil
	},
}

func init() {
	standupCmd.Flags().StringVar(&standupFlags.date, "date", "", "Day to report on (YYYY-MM-DD), default today")
	rootCmd.AddCommand(standupCmd)
}
