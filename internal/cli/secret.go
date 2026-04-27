package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var secretCmd = &cobra.Command{
	Use:   "secret",
	Short: "Manage encrypted secrets (CalDAV password, etc.)",
}

var secretSetCmd = &cobra.Command{
	Use:   "set <name>",
	Short: "Read a value from stdin and store it encrypted",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		fmt.Fprintf(os.Stderr, "Enter value for %q (input is read from stdin until newline):\n", args[0])
		reader := bufio.NewReader(os.Stdin)
		raw, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
		val := strings.TrimRight(raw, "\r\n")

		c, err := newClient()
		if err != nil {
			return err
		}
		if err := c.SetSecret(context.Background(), args[0], val); err != nil {
			return err
		}
		fmt.Println("ok")
		return nil
	},
}

var secretListCmd = &cobra.Command{
	Use:   "list",
	Short: "List known secret names (no values)",
	RunE: func(_ *cobra.Command, _ []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		list, err := c.ListSecretNames(context.Background())
		if err != nil {
			return err
		}
		if len(list) == 0 {
			fmt.Println("(no secrets)")
			return nil
		}
		for _, s := range list {
			fmt.Println(s.Name)
		}
		return nil
	},
}

func init() {
	secretCmd.AddCommand(secretSetCmd, secretListCmd)
	rootCmd.AddCommand(secretCmd)
}
