package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/urfave/cli"

	"github.com/arigatomachine/cli/api"
	"github.com/arigatomachine/cli/identity"
)

func init() {
	invites := cli.Command{
		Name:     "invites",
		Usage:    "View and accept organization invites",
		Category: "ORGANIZATIONS",
		Subcommands: []cli.Command{
			{
				Name:      "send",
				Usage:     "Send an invitation to join an organization to an email address",
				ArgsUsage: "<email>",
				Flags: []cli.Flag{
					OrgFlag("org to invite user to"),
					newSlicePlaceholder("team, o", "TEAM", "team to add user to", nil),
				},
				Action: Chain(EnsureDaemon, EnsureSession, sendInvite),
			},
		},
	}
	Cmds = append(Cmds, invites)
}

const orgInviteFailed = "Could not send invitation to org, please try again."

func sendInvite(ctx *cli.Context) error {
	spacer := "    "
	usage := "Usage:\n" + spacer + ctx.App.HelpName + " " + ctx.Command.Name + " [command options] " + ctx.Command.ArgsUsage
	if !ctx.IsSet("org") {
		text := "Missing --org flag\n\n"
		text += usage
		return cli.NewExitError(text, -1)
	}

	args := ctx.Args()
	if len(args) < 1 {
		text := "Missing email\n\n"
		text += usage
		return cli.NewExitError(text, -1)
	}

	var teamNames []string
	if ctx.IsSet("team") {
		teamNames = ctx.StringSlice("team")
	} else {
		teamNames = append(teamNames, "member")
	}

	email := ctx.Args()[0]
	if email == "" {
		text := "Missing email\n\n"
		text += usage
		return cli.NewExitError(text, -1)
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	client := api.NewClient(cfg)

	org, err := client.Orgs.GetByName(context.Background(), ctx.String("org"))
	if err != nil {
		return cli.NewExitError(orgInviteFailed, -1)
	}
	if org == nil {
		return cli.NewExitError("Org not found", -1)
	}

	// Identify the user attempting the command
	user, err := client.Users.Self(context.Background())
	if err != nil {
		return cli.NewExitError(orgInviteFailed, -1)
	}

	// Retrieve teams for our target org
	teams, err := client.Teams.GetByOrg(context.Background(), org.ID)
	if err != nil {
		return cli.NewExitError(orgInviteFailed, -1)
	}

	var matchTeams []string
	matchTeams = ctx.StringSlice("team")
	if len(ctx.StringSlice("team")) < 1 {
		matchTeams = append(matchTeams, "member")
	}

	// Verify all team names supplied exist for this org
	teamIDs := make([]identity.ID, len(matchTeams))
	var missingTeams []string

TeamSearch:
	for i, teamName := range matchTeams {
		for _, team := range teams {
			if team.Body.Name == teamName {
				teamIDs[i] = *team.ID
				continue TeamSearch
			}
		}
		missingTeams = append(missingTeams, teamName)
	}

	// One of the supplied teams is not known to this org
	if len(missingTeams) > 0 {
		missingTeamNames := strings.Join(missingTeams, ", ")
		return cli.NewExitError("Unknown team(s): "+missingTeamNames, -1)
	}
	if len(teamIDs) < 1 {
		return cli.NewExitError(orgInviteFailed, -1)
	}

	err = client.Invites.Send(context.Background(), email, *org.ID, *user.ID, teamIDs)
	if err != nil {
		if strings.Contains(err.Error(), "resource exists") {
			return cli.NewExitError(email+" has already been invited to the "+org.Body.Name+" org", -1)
		}
		return cli.NewExitError(orgInviteFailed, -1)
	}

	fmt.Println("Invitation to join the " + org.Body.Name + " organization has been sent to " + email + ".")
	fmt.Println("\nThey will be added to the following teams once their invite has been confirmed:")
	fmt.Println("\n\t" + strings.Join(matchTeams, "\n\t"))
	fmt.Println("\nThey will receive an e-mail with instructions.")

	return nil
}
