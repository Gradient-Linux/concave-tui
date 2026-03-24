package model

import (
	"context"
	"time"

	tuiauth "github.com/Gradient-Linux/concave-tui/internal/auth"
	apiclient "github.com/Gradient-Linux/concave-tui/internal/client"
)

var (
	loadSessionFn  = tuiauth.LoadSession
	saveSessionFn  = tuiauth.SaveSession
	clearSessionFn = tuiauth.ClearSession

	newClientFn = func() *apiclient.Client {
		return apiclient.New(apiclient.DefaultBaseURL())
	}
	sharedClient = newClientFn()

	apiLoginFn = func(ctx context.Context, username, password string) (tuiauth.Session, error) {
		return sharedClient.Login(ctx, username, password)
	}
	apiRefreshFn = func(ctx context.Context) (tuiauth.Session, error) {
		return sharedClient.Refresh(ctx)
	}
	apiSuitesFn = func(ctx context.Context) ([]apiclient.SuiteSummary, error) {
		return sharedClient.ListSuites(ctx)
	}
	apiSuiteFn = func(ctx context.Context, name string) (apiclient.SuiteSummary, error) {
		return sharedClient.GetSuite(ctx, name)
	}
	apiWorkspaceFn = func(ctx context.Context) (apiclient.WorkspacePayload, error) {
		return sharedClient.Workspace(ctx)
	}
	apiWorkspaceBackupFn = func(ctx context.Context) (string, error) {
		return sharedClient.WorkspaceBackup(ctx)
	}
	apiWorkspaceCleanFn = func(ctx context.Context) (string, error) {
		return sharedClient.WorkspaceClean(ctx)
	}
	apiDoctorFn = func(ctx context.Context) ([]apiclient.DoctorCheck, error) {
		return sharedClient.Doctor(ctx)
	}
	apiSystemInfoFn = func(ctx context.Context) (apiclient.SystemInfo, error) {
		return sharedClient.SystemInfo(ctx)
	}
	apiUsersActivityFn = func(ctx context.Context) ([]apiclient.UserActivity, error) {
		return sharedClient.UsersActivity(ctx)
	}
	apiResolverStatusFn = func(ctx context.Context) (apiclient.ResolverStatus, error) {
		return sharedClient.ResolverStatus(ctx)
	}
	apiNodeStatusFn = func(ctx context.Context) (apiclient.FleetNode, error) {
		return sharedClient.NodeStatus(ctx)
	}
	apiFleetStatusFn = func(ctx context.Context) (apiclient.FleetResponse, error) {
		return sharedClient.FleetStatus(ctx)
	}
	apiTeamsFn = func(ctx context.Context) (apiclient.TeamsResponse, error) {
		return sharedClient.Teams(ctx)
	}
	apiSystemActionFn = func(ctx context.Context, action string) error {
		return sharedClient.SystemAction(ctx, action)
	}
	apiSuiteActionFn = func(ctx context.Context, name, action string, body any) (string, error) {
		return sharedClient.StartSuiteAction(ctx, name, action, body)
	}
	apiJobFn = func(ctx context.Context, id string) (apiclient.JobSnapshot, error) {
		return sharedClient.Job(ctx, id)
	}
	apiLabURLFn = func(ctx context.Context, name string) (string, error) {
		return sharedClient.LabURL(ctx, name)
	}
	apiChangelogFn = func(ctx context.Context, name string) (apiclient.ChangelogResponse, error) {
		return sharedClient.Changelog(ctx, name)
	}
	apiLogsDialFn = func(ctx context.Context, suiteName, container string) (logStream, error) {
		conn, err := sharedClient.DialLogs(ctx, suiteName, container)
		if err != nil {
			return logStream{}, err
		}
		ch := make(chan dockerLogEvent, 128)
		done := make(chan struct{})
		go func() {
			defer close(ch)
			defer close(done)
			for {
				var event apiclient.WebsocketEvent
				if err := conn.ReadJSON(&event); err != nil {
					ch <- dockerLogEvent{done: true, err: err}
					return
				}
				if event.Type == "line" && event.Line != "" {
					ch <- dockerLogEvent{line: event.Line}
				}
			}
		}()
		return logStream{
			cancel: func() {
				_ = conn.Close()
				select {
				case <-done:
				case <-time.After(time.Second):
				}
			},
			ch: ch,
		}, nil
	}
)

func setClientSession(session tuiauth.Session) {
	if sharedClient == nil {
		sharedClient = newClientFn()
	}
	sharedClient.SetToken(session.Token)
}

func resetClientSession() {
	if sharedClient == nil {
		sharedClient = newClientFn()
		return
	}
	sharedClient.SetToken("")
}

func replaceSharedClient(client *apiclient.Client) {
	if client == nil {
		sharedClient = newClientFn()
		return
	}
	sharedClient = client
}
