# DesignHub Google Sign-in Setup

This guide sets up Google sign-in for one DesignHub installation. Each installation (each company) uses its own Google OAuth client and its own settings.

## Before you start

| Requirement | Detail |
|---|---|
| DesignHub version | Includes PR #6 (Google sign-in). Pull the image after the merge: `sudo docker compose pull && sudo docker compose up -d` |
| Public HTTPS address | A hostname that reaches DesignHub over HTTPS, for example `https://designhub.example.com`. Google does not accept a plain IP address or `http`. |
| Allowed addresses | The public hostname is listed in DesignHub, Admin, System, Allowed addresses |
| Google account | An account that can create a Google Cloud project. For a Google Workspace company, use an account in that Workspace. |
| Super Admin email | Know which Google address the DesignHub Super Admin will use |

Always open DesignHub through the public hostname when signing in with Google. A Google sign-in started from the local IP address fails, because Google returns to the public hostname.

## Part 1. Google Cloud

### 1.1 Create a project

1. Open https://console.cloud.google.com and sign in.
2. In the project picker at the top, select **New project**.
3. Name it, for example `DesignHub`. For a Workspace company, choose the company organization as the location.
4. Select **Create**, then switch to the new project.

### 1.2 Configure the Google Auth Platform

1. Open the menu, then **Google Auth Platform** (https://console.cloud.google.com/auth/overview).
2. If prompted, select **Get started**.
3. **App information:** app name `DesignHub`, user support email (your address).
4. **Audience:** choose one:

| Choice | Use when | Effect |
|---|---|---|
| Internal | Everyone who will sign in is in one Google Workspace organization | Only that organization's accounts can sign in. No Google review. |
| External | Personal Google accounts (such as gmail.com) will also sign in | Any Google account can reach the consent screen. DesignHub's allowed lists still decide who gets in. |

5. **Contact information:** your email address.
6. Accept the policy and select **Create**.

If you chose **External**:

1. Open **Audience**.
2. While the app is in **Testing**, only the addresses listed under **Test users** (up to 100) can sign in. Add each person, or select **Publish app** to move it to **In production**.
3. DesignHub asks only for `openid`, `email` and `profile`. These are non-sensitive scopes. To avoid triggering a brand review, do not upload a logo on the **Branding** page.

### 1.3 Data access

1. Open **Data Access**.
2. Select **Add or remove scopes**.
3. Select `openid`, `.../auth/userinfo.email` and `.../auth/userinfo.profile`.
4. Select **Update**, then **Save**.

### 1.4 Create the OAuth client

1. Open **Clients**, then **Create client**.
2. **Application type:** Web application.
3. **Name:** `DesignHub`.
4. **Authorized JavaScript origins:** leave empty.
5. **Authorized redirect URIs:** select **Add URI** and enter the redirect URI shown in DesignHub (Part 2, step 3). It has the form `https://designhub.example.com/api/auth/google/callback`. It must match exactly: `https`, same hostname, no trailing slash.
6. Select **Create**.
7. Copy the **Client ID** and **Client secret** now. The full secret is shown only at creation. Store it in your password manager.

A new or changed redirect URI can take from five minutes to a few hours to start working.

## Part 2. DesignHub

Sign in as the Super Admin with your password.

1. Open **Admin**, **System**.
2. In **Allowed addresses**, make sure the public hostname is listed (no `https://`). Save.
3. In **Google sign-in**, enter the **Public address**, for example `https://designhub.example.com`, and select **Save**. The **Redirect URI** box now shows the address to register in Part 1.4, step 5.
4. Enter the **Client ID** and **Client secret**.
5. Enter **Allowed Google Workspace domains**, one per line, for example `example.com`. Only Google Workspace accounts of these domains are admitted.
6. Enter **Allowed individual addresses**, one per line, for people outside those domains, for example `someone@gmail.com`.
7. Select **Save**.
8. Tick **Show Google sign-in**.

### Link the Super Admin

1. Open **Settings**, **Profile**. Set **Email** to the Google address you will sign in with. Save.
2. Sign out.
3. Open the public address and select **Sign in with Google**. Choose that Google account.
4. DesignHub opens signed in as the Super Admin. From now on this account signs in with Google only.
5. Open **Settings**, **Security** and set a **Signing PIN**. It is used for change order approvals, releases and NCR decisions.

### Turn off password sign-in

When Google sign-in works for you:

1. Open **Admin**, **System**, **Google sign-in**.
2. Tick **Turn off password sign-in** and confirm.

Every account, including Super Admins, now signs in with Google.

## Part 3. People

| Situation | What happens |
|---|---|
| New person in an allowed domain signs in | Account created as Viewer with no space access. An Admin then sets the library role and space roles in Admin, Users. |
| Existing local account | Set the account's email to the person's Google address before they sign in. Their first Google sign-in links to it and keeps their roles. |
| Person leaves | Disable the account in Admin, Users. Removing their address or domain from the allowed lists also blocks them. |
| Forgotten signing PIN | Admin, Users, the person, **Clear signing PIN**. They set a new one in Settings, Security. |

## Part 4. Recovery

If no Super Admin can sign in with Google (Google outage, lost account, wrong settings), run this on the Docker host:

```bash
cd /opt/designhub
sudo docker compose exec -it designhub designhub recovery
```

Enter the Super Admin username and a new password. That account can sign in once with the password within one hour, even with password sign-in turned off. Use **Recovery sign-in** on the sign-in screen.

## Part 5. Check

| Check | Expected |
|---|---|
| Sign in with an allowed Workspace account | Signed in |
| Sign in with a personal account that is not listed | "That Google account is not allowed to use DesignHub." |
| Password sign-in for a normal user after password sign-in is turned off | "Password sign-in is turned off. Sign in with Google." |
| Approve a change order as a Google user | Asks for the signing PIN |

## Troubleshooting

| Message or symptom | Cause | Fix |
|---|---|---|
| Google shows `redirect_uri_mismatch` | The redirect URI in Google differs from DesignHub's | Copy the Redirect URI from Admin, System into the Google client exactly. Wait a few minutes. |
| Google shows "Access blocked: app is restricted to users within its organization" | Audience is Internal and the account is outside the organization | Use a company account, or change the Audience to External |
| Google shows the person is not a test user | Audience is External and the app is in Testing | Add them under Audience, Test users, or publish the app |
| "That Google account is not allowed to use DesignHub." | The address is not in an allowed domain or the individual list, or a personal account uses a company email address | Add the domain or the address in Admin, System |
| "More than one DesignHub account uses this email address." | Two accounts share the email | Change one account's email in Admin, Users |
| "Google sign-in failed." | The client secret is wrong, or the server cannot reach Google | Re-enter the client secret. Check the server's internet access. |
| "The sign-in request expired." | Sign-in was started from the local IP address, more than 10 minutes passed, or the server restarted during sign-in | Open DesignHub through the public hostname and sign in again |
