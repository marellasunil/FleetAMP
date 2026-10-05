# Password management and recovery

FleetAMP supports three credential workflows:

- Users change their own password from **My account** by entering the current password and a new password. All of that user's sessions are revoked.
- An Admin resets another user or Admin from **Administration → Users**. The Admin must re-enter their own password. The target receives a temporary password, all target sessions are revoked, and FleetAMP requires a new password at the next sign-in.
- A host administrator can issue a short-lived recovery token when no FleetAMP Admin can sign in.

An Admin cannot reset their own password through the user-administration dialog; they use **My account** instead. FleetAMP also retains the existing protection that prevents disabling or demoting the last enabled Admin.

## Break-glass recovery

Generate a 15-minute, single-use token on the FleetAMP host:

```bash
fleetamp admin recovery-token \
  --database /var/lib/fleetamp/fleetamp.db \
  --username admin
```

The command prints JSON containing `recovery_token`, `expires_at`, and `recovery_path`. Open `/recover`, then enter the username, token, and new password. Generating and consuming a token are recorded in the audit log; the token value itself is never recorded.

Use `--expires` to choose a shorter lifetime. The maximum is one hour. Issuing another token for the same user invalidates the previous token.

The command requires filesystem access to the FleetAMP database. Run it only through an authorized host or Kubernetes administrative workflow. Do not paste a recovery token into tickets, logs, shell history, or chat.
