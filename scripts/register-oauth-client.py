#!/usr/bin/env python3
"""
Register enclii-cli OAuth client in Janua

This script creates the OAuth client needed for CLI login.
Run once with admin credentials to bootstrap the client.

Usage:
  JANUA_ADMIN_EMAIL=admin@madfam.io JANUA_ADMIN_PASSWORD=xxx python scripts/register-oauth-client.py

Or run interactively:
  python scripts/register-oauth-client.py
"""

import os
import sys
import getpass
import requests

JANUA_API = os.getenv("JANUA_API_URL", "https://api.janua.dev")

# OAuth client configuration for enclii-cli
OAUTH_CLIENT_CONFIG = {
    "name": "Enclii CLI",
    "description": "Official Enclii command-line interface for deployment and management",
    "redirect_uris": [
        "http://127.0.0.1/callback",  # Localhost callback for CLI
    ],
    "allowed_scopes": ["openid", "profile", "email", "offline_access"],
    "grant_types": ["authorization_code", "refresh_token"],
    "is_confidential": False,  # Public client (uses PKCE)
    "website_url": "https://enclii.dev",
}


def login(email: str, password: str) -> str:
    """Login to Janua and return access token"""
    resp = requests.post(
        f"{JANUA_API}/api/v1/auth/login",
        json={"email": email, "password": password},
    )

    if resp.status_code != 200:
        error = resp.json().get("error", {})
        raise Exception(f"Login failed: {error.get('message', resp.text)}")

    data = resp.json()
    return data.get("access_token") or data.get("token")


def create_oauth_client(token: str) -> dict:
    """Create the OAuth client and return client details"""
    resp = requests.post(
        f"{JANUA_API}/api/v1/oauth/clients",
        headers={"Authorization": f"Bearer {token}"},
        json=OAUTH_CLIENT_CONFIG,
    )

    if resp.status_code == 201:
        return resp.json()
    elif resp.status_code == 409:
        raise Exception("OAuth client already exists")
    else:
        error = resp.json().get("error", {})
        raise Exception(f"Failed to create client: {error.get('message', resp.text)}")


def main():
    print("Enclii CLI OAuth Client Registration")
    print("=" * 40)
    print()

    # Get credentials
    email = os.getenv("JANUA_ADMIN_EMAIL")
    password = os.getenv("JANUA_ADMIN_PASSWORD")

    if not email:
        email = input("Janua admin email: ")
    if not password:
        password = getpass.getpass("Janua admin password: ")

    print()
    print(f"Logging in as {email}...")

    try:
        token = login(email, password)
        print("Login successful")
    except Exception as e:
        print(f"Error: {e}")
        sys.exit(1)

    print()
    print("Creating OAuth client...")
    print(f"  Name: {OAUTH_CLIENT_CONFIG['name']}")
    print(f"  Public client (PKCE): {not OAUTH_CLIENT_CONFIG['is_confidential']}")
    print(f"  Scopes: {', '.join(OAUTH_CLIENT_CONFIG['allowed_scopes'])}")
    print()

    try:
        client = create_oauth_client(token)
        print("OAuth client created successfully!")
        print()
        print("Client Details:")
        print(f"  client_id: {client.get('client_id')}")
        if client.get('client_secret'):
            print(f"  client_secret: {client.get('client_secret')}")
            print()
            print("  NOTE: Save the client_secret now - it won't be shown again!")
        print()
        print("The enclii-cli OAuth client is now registered.")
        print("Users can authenticate with: enclii login")

    except Exception as e:
        print(f"Error: {e}")
        sys.exit(1)


if __name__ == "__main__":
    main()
