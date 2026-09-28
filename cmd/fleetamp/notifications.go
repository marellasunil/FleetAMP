package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/mail"
	"net/smtp"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/groups"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type approvalNotifier struct {
	addr      string
	username  string
	password  string
	from      string
	publicURL string
	users     userStore
}

func loadApprovalNotifier(users userStore) (*approvalNotifier, error) {
	addr := strings.TrimSpace(os.Getenv("FLEETAMP_SMTP_ADDR"))
	if addr == "" {
		return &approvalNotifier{users: users}, nil
	}
	from := strings.TrimSpace(os.Getenv("FLEETAMP_SMTP_FROM"))
	if from == "" {
		return nil, fmt.Errorf("FLEETAMP_SMTP_FROM is required when FLEETAMP_SMTP_ADDR is configured")
	}
	parsedFrom, err := mail.ParseAddress(from)
	if err != nil || !strings.EqualFold(parsedFrom.Address, from) {
		return nil, fmt.Errorf("FLEETAMP_SMTP_FROM must be a plain valid email address")
	}
	if _, _, err := net.SplitHostPort(addr); err != nil {
		return nil, fmt.Errorf("FLEETAMP_SMTP_ADDR must use host:port format: %w", err)
	}
	return &approvalNotifier{
		addr: addr, username: strings.TrimSpace(os.Getenv("FLEETAMP_SMTP_USERNAME")),
		password: os.Getenv("FLEETAMP_SMTP_PASSWORD"), from: from,
		publicURL: strings.TrimRight(strings.TrimSpace(os.Getenv("FLEETAMP_PUBLIC_URL")), "/"), users: users,
	}, nil
}

func (n *approvalNotifier) enabled() bool { return n != nil && n.addr != "" }

func (n *approvalNotifier) notify(ctx context.Context, event string, request *configs.GroupDeploymentRequest, group *groups.Group) {
	if !n.enabled() || request == nil || group == nil {
		return
	}
	users, err := n.users.List(ctx)
	if err != nil {
		slog.Error("list approval notification recipients", "component", "notifications", "error", err)
		return
	}
	ownerSet := make(map[string]struct{}, len(group.Owners))
	for _, owner := range group.Owners {
		ownerSet[strings.ToLower(owner)] = struct{}{}
	}
	recipientSet := map[string]struct{}{}
	for _, user := range users {
		_, owner := ownerSet[strings.ToLower(user.Username)]
		if user.Enabled && user.Email != "" && (user.Role == string(roleAdmin) || owner) {
			recipientSet[user.Email] = struct{}{}
		}
	}
	recipients := make([]string, 0, len(recipientSet))
	for recipient := range recipientSet {
		recipients = append(recipients, recipient)
	}
	sort.Strings(recipients)
	if len(recipients) == 0 {
		return
	}
	safeHeader := strings.NewReplacer("\r", " ", "\n", " ").Replace
	subject := safeHeader(fmt.Sprintf("[FleetAMP] Approval %s: %s v%s", event, request.ConfigurationName, request.ConfigurationVersion))
	link := ""
	if n.publicURL != "" {
		link = "\nReview: " + n.publicURL + "/approvals/" + url.PathEscape(request.ID)
	}
	body := fmt.Sprintf("FleetAMP approval request status: %s\n\nGroup: %s\nConfiguration: %s\nVersion: %s\nCollectors: %d\nRequester: %s\nAssigned reviewer: %s\nChange reason: %s\nStatus: %s\nExpires: %s%s\n",
		event, request.GroupName, request.ConfigurationName, request.ConfigurationVersion, len(request.Targets), request.RequestedBy, request.AssignedReviewer, request.ChangeReason, request.Status,
		request.ExpiresAt.UTC().Format(time.RFC3339), link)
	message := []byte("To: " + strings.Join(recipients, ", ") + "\r\nSubject: " + subject + "\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n" + body)
	go func() {
		host, _, _ := net.SplitHostPort(n.addr)
		var auth smtp.Auth
		if n.username != "" {
			auth = smtp.PlainAuth("", n.username, n.password, host)
		}
		if err := smtp.SendMail(n.addr, auth, n.from, recipients, message); err != nil {
			slog.Error("send approval notification", "component", "notifications", "event", event, "request_id", request.ID, "error", err)
		}
	}()
}

func runApprovalExpiryLoop(ctx context.Context, requestStore storage.GroupDeploymentRequestStore, groupStore storage.GroupStore, notifier *approvalNotifier) {
	expireApprovalRequests(ctx, requestStore, groupStore, notifier)
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			expireApprovalRequests(ctx, requestStore, groupStore, notifier)
		}
	}
}

func expireApprovalRequests(ctx context.Context, requestStore storage.GroupDeploymentRequestStore, groupStore storage.GroupStore, notifier *approvalNotifier) {
	requests, err := requestStore.ExpirePending(ctx, time.Now().UTC())
	if err != nil && ctx.Err() == nil {
		slog.Error("expire approval requests", "component", "approvals", "error", err)
		return
	}
	for _, request := range requests {
		group, err := groupStore.Get(ctx, request.GroupID)
		if err == nil {
			notifier.notify(ctx, "expired", request, group)
		}
	}
}
