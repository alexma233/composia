package cli

import (
	"fmt"

	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
)

func (application *app) runSecret(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: composia secret <get|edit|update>")
	}
	switch args[0] {
	case "get":
		return application.runSecretGet(args[1:])
	case "edit":
		return application.runSecretEdit(args[1:])
	case "update":
		return application.runSecretUpdate(args[1:])
	default:
		return fmt.Errorf("unknown secret command %q", args[0])
	}
}

func (application *app) runSecretGet(args []string) error {
	if err := requireArgs(args, 2, "composia secret get <service> <file>"); err != nil {
		return err
	}
	response, err := application.client.secrets.GetSecret(application.ctx, newRequest(&controllerv1.GetSecretRequest{ServiceName: args[0], FilePath: args[1]}))
	if err != nil {
		return err
	}
	if application.cfg.json {
		return application.printMessage(response.Msg)
	}
	_, err = fmt.Fprint(application.out, response.Msg.GetContent())
	return err
}

func (application *app) runSecretEdit(args []string) error {
	fs := newCommandFlagSet("secret edit")
	message := fs.String("message", "", "commit message")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 2, "composia secret edit [--message text] <service> <file>"); err != nil {
		return err
	}
	serviceName := fs.Arg(0)
	filePath := fs.Arg(1)
	baseRevision, err := application.currentRepoRevision()
	if err != nil {
		return err
	}
	secret, err := application.client.secrets.GetSecret(application.ctx, newRequest(&controllerv1.GetSecretRequest{ServiceName: serviceName, FilePath: filePath}))
	if err != nil {
		return err
	}
	updatedContent, changed, err := editText(application.ctx, secret.Msg.GetContent(), "composia-secret-*", 0o600)
	if err != nil {
		return err
	}
	if !changed {
		_, err := fmt.Fprintln(application.out, "unchanged")
		return err
	}
	response, err := application.client.secrets.UpdateSecret(application.ctx, newRequest(&controllerv1.UpdateSecretRequest{ServiceName: serviceName, FilePath: filePath, Content: updatedContent, BaseRevision: baseRevision, CommitMessage: *message}))
	if err != nil {
		return repoWriteError(err)
	}
	if application.cfg.json {
		return application.printMessage(response.Msg)
	}
	return application.printRepoWriteResult(repoWriteResult{
		commitID:             response.Msg.GetCommitId(),
		syncStatus:           response.Msg.GetSyncStatus(),
		pushError:            response.Msg.GetPushError(),
		lastSuccessfulPullAt: response.Msg.GetLastSuccessfulPullAt(),
	})
}

func (application *app) runSecretUpdate(args []string) error {
	fs := newCommandFlagSet("secret update")
	fileSource := fs.String("file", "", "plaintext file to read; use - for stdin")
	message := fs.String("message", "", "commit message")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 2, "composia secret update --file file [--message text] <service> <file>"); err != nil {
		return err
	}
	if *fileSource == "" {
		return errorsWithUsage("file is required", "composia secret update --file file [--message text] <service> <file>")
	}
	content, err := readContentSource(*fileSource)
	if err != nil {
		return err
	}
	baseRevision, err := application.currentRepoRevision()
	if err != nil {
		return err
	}
	response, err := application.client.secrets.UpdateSecret(application.ctx, newRequest(&controllerv1.UpdateSecretRequest{ServiceName: fs.Arg(0), FilePath: fs.Arg(1), Content: content, BaseRevision: baseRevision, CommitMessage: *message}))
	if err != nil {
		return repoWriteError(err)
	}
	if application.cfg.json {
		return application.printMessage(response.Msg)
	}
	return application.printRepoWriteResult(repoWriteResult{
		commitID:             response.Msg.GetCommitId(),
		syncStatus:           response.Msg.GetSyncStatus(),
		pushError:            response.Msg.GetPushError(),
		lastSuccessfulPullAt: response.Msg.GetLastSuccessfulPullAt(),
	})
}
