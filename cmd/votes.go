package cmd

import "github.com/spf13/cobra"

const voteHelp = `Sends many votes from different users.

This command requires, that there are many user created at the
backend. You can use the command "create_users" for this job.

Example:

performance votes motion --amount 100 --poll_id 42

performance votes assignment --amount 100 --poll_id 42`

func cmdVotes(cfg *config) *cobra.Command {
	cmd := cobra.Command{
		Use:       "votes",
		Short:     "Sends many votes from different users",
		ValidArgs: []string{"motion", "assignment", "m", "a"},
		Args:      cobra.ExactValidArgs(1),
		Long:      voteHelp,
	}
	// amount := cmd.Flags().IntP("amount", "n", 10, "Amount of users to use.")
	// pollID := cmd.Flags().IntP("poll_id", "i", 1, "ID of the poll to use.")
	// interrupt := cmd.Flags().Bool("interrupt", false, "Wait for a user input after login.")
	// loginRetry := cmd.Flags().IntP("login_retry", "r", 3, "Retries send login requests before returning an error.")
	// choice := cmd.Flags().IntP("choice", "c", 0, "Amount of answers per vote.")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return nil
	}

	return &cmd

}
