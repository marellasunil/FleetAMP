# Password management and recovery

OTelPlane supports three credential workflows:

- Users change their own password from **My account** by entering the current password and a new password. All of that user's sessions are revoked.
- An Admin resets another user or Admin from **Administration → Users**. The Admin must re-enter their own password. The target receives a temporary password, all target sessions are revoked, and OTelPlane requires a new password at the next sign-in.
- A host administrator can issue a short-lived recovery token when no OTelPlane Admin can sign in.

An Admin cannot reset their own password through the user-administration dialog; they use **My account** instead. OTelPlane also retains the existing protection that prevents disabling or demoting the last enabled Admin.

## Break-glass recovery

Generate a 15-minute, single-use token on the OTelPlane host:

```bash
otelplane admin recovery-token \
  --database /var/lib/otelplane/otelplane.db \
  --username admin
```

The command prints JSON containing `recovery_token`, `expires_at`, and `recovery_path`. Open `/recover`, then enter the username, token, and new password. Generating and consuming a token are recorded in the audit log; the token value itself is never recorded.

Use `--expires` to choose a shorter lifetime. The maximum is one hour. Issuing another token for the same user invalidates the previous token.

The command requires filesystem access to the OTelPlane database. Run it only through an authorized host or Kubernetes administrative workflow. Do not paste a recovery token into tickets, logs, shell history, or chat.
