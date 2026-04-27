package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/paths"
)

var todoCmd = &cobra.Command{
	Use:   "todo",
	Short: "Manage todos",
}

var todoAddFlags struct {
	priority string
	tags     []string
	due      string
	notes    string
}

var todoAddCmd = &cobra.Command{
	Use:   "add <title>",
	Short: "Add a new todo",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		title := strings.Join(args, " ")
		req := api.CreateTodoRequest{
			Title:    title,
			Notes:    todoAddFlags.notes,
			Priority: parsePriority(todoAddFlags.priority),
			Tags:     todoAddFlags.tags,
		}
		if todoAddFlags.due != "" {
			ts, err := parseDue(todoAddFlags.due)
			if err != nil {
				return err
			}
			req.DueAt = &ts
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		t, err := c.CreateTodo(context.Background(), req)
		if err != nil {
			return err
		}
		fmt.Printf("created %s: %s\n", t.ID, t.Title)
		return nil
	},
}

var todoListCmd = &cobra.Command{
	Use:   "list",
	Short: "List todos",
	RunE: func(cmd *cobra.Command, _ []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		all, _ := cmd.Flags().GetBool("all")
		todos, err := c.ListTodos(context.Background(), all)
		if err != nil {
			return err
		}
		if len(todos) == 0 {
			fmt.Println("(no todos)")
			return nil
		}
		for _, t := range todos {
			due := ""
			if t.DueAt != nil {
				due = " [due " + time.Unix(*t.DueAt, 0).Format("2006-01-02") + "]"
			}
			tags := ""
			if len(t.Tags) > 0 {
				tags = " #" + strings.Join(t.Tags, " #")
			}
			suffix := t.ID
			if len(suffix) > 6 {
				suffix = suffix[len(suffix)-6:]
			}
			fmt.Printf("%s [%s] (%s) %s%s%s\n", suffix, priorityStr(t.Priority), t.Status, t.Title, due, tags)
		}
		return nil
	},
}

var todoDoneCmd = &cobra.Command{
	Use:   "done <id-suffix>",
	Short: "Mark a todo as done",
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
		done := api.StatusDone
		_, err = c.UpdateTodo(context.Background(), id, api.UpdateTodoRequest{Status: &done})
		if err != nil {
			return err
		}
		fmt.Println("done")
		return nil
	},
}

var todoRmCmd = &cobra.Command{
	Use:   "rm <id-suffix>",
	Short: "Delete a todo",
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
		if err := c.DeleteTodo(context.Background(), id); err != nil {
			return err
		}
		fmt.Println("deleted")
		return nil
	},
}

func init() {
	todoAddCmd.Flags().StringVarP(&todoAddFlags.priority, "priority", "p", "normal", "low|normal|high|urgent")
	todoAddCmd.Flags().StringSliceVarP(&todoAddFlags.tags, "tag", "t", nil, "tag (repeatable)")
	todoAddCmd.Flags().StringVar(&todoAddFlags.due, "due", "", "YYYY-MM-DD or unix seconds")
	todoAddCmd.Flags().StringVarP(&todoAddFlags.notes, "notes", "n", "", "notes (markdown)")
	todoListCmd.Flags().BoolP("all", "a", false, "include done/cancelled")
	todoCmd.AddCommand(todoAddCmd, todoListCmd, todoDoneCmd, todoRmCmd)
	rootCmd.AddCommand(todoCmd)
}

func newClient() (*Client, error) {
	p, err := paths.New()
	if err != nil {
		return nil, err
	}
	c := NewClient(p.SocketFile)
	if err := c.Health(context.Background()); err != nil {
		return nil, fmt.Errorf("daemon not reachable (run `cockpit start`): %w", err)
	}
	return c, nil
}

func resolveID(c *Client, suffix string) (string, error) {
	todos, err := c.ListTodos(context.Background(), true)
	if err != nil {
		return "", err
	}
	var matches []string
	for _, t := range todos {
		if strings.HasSuffix(t.ID, suffix) {
			matches = append(matches, t.ID)
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no todo matching %q", suffix)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("ambiguous suffix %q matches %d todos", suffix, len(matches))
	}
	return matches[0], nil
}

func parsePriority(s string) api.Priority {
	switch strings.ToLower(s) {
	case "low":
		return api.PriorityLow
	case "high":
		return api.PriorityHigh
	case "urgent":
		return api.PriorityUrgent
	default:
		return api.PriorityNormal
	}
}

func priorityStr(p api.Priority) string {
	switch p {
	case api.PriorityLow:
		return "low"
	case api.PriorityHigh:
		return "high"
	case api.PriorityUrgent:
		return "urgent"
	default:
		return "normal"
	}
}

func parseDue(s string) (int64, error) {
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n, nil
	}
	t, err := time.ParseInLocation("2006-01-02", s, time.Local)
	if err != nil {
		return 0, fmt.Errorf("invalid due value %q (expected YYYY-MM-DD or unix seconds)", s)
	}
	return t.Add(18 * time.Hour).Unix(), nil
}
