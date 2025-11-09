#!/usr/bin/env python3
"""
Post or update coverage report comment on GitHub Pull Request.

This script generates a formatted coverage report comment and posts it to a PR,
or updates an existing coverage comment if one already exists.
"""

import os
import sys
import json
import requests
from typing import Optional


def read_file(file_path: str) -> str:
    """Read file content."""
    try:
        with open(file_path, 'r') as f:
            return f.read()
    except FileNotFoundError:
        print(f"Warning: File {file_path} not found", file=sys.stderr)
        return ""


def generate_comment_body(coverage: str, emoji: str, status: str,
                          badge_color: str, coverage_report_path: str) -> str:
    """
    Generate the PR comment body.

    Args:
        coverage: Coverage percentage (e.g., "75.5%")
        emoji: Status emoji
        status: Status text
        badge_color: Badge color
        coverage_report_path: Path to detailed coverage report

    Returns:
        Formatted comment body in markdown
    """
    coverage_report = read_file(coverage_report_path)

    # URL encode the coverage percentage for the badge
    coverage_encoded = coverage.replace('%', '%25')

    comment = f"""## {emoji} Go Test Coverage Report

**Total Coverage:** `{coverage}` ({status})

![Coverage](https://img.shields.io/badge/coverage-{coverage_encoded}-{badge_color})

<details>
<summary>📊 Detailed Coverage Report (click to expand)</summary>

{coverage_report}

</details>

### Coverage Guidelines
- 🟢 >= 80%: Excellent
- 🟡 >= 60%: Good
- 🟠 >= 40%: Fair
- 🔴 < 40%: Needs improvement

---
*This is an automated coverage report. The coverage requirement is advisory and does not block PR merging.*
"""
    return comment


def find_existing_comment(token: str, repo: str, pr_number: int) -> Optional[int]:
    """
    Find existing coverage comment in the PR.

    Args:
        token: GitHub token
        repo: Repository in format "owner/repo"
        pr_number: Pull request number

    Returns:
        Comment ID if found, None otherwise
    """
    url = f"https://api.github.com/repos/{repo}/issues/{pr_number}/comments"
    headers = {
        'Authorization': f'token {token}',
        'Accept': 'application/vnd.github.v3+json'
    }

    try:
        response = requests.get(url, headers=headers)
        response.raise_for_status()
        comments = response.json()

        # Look for existing coverage comment
        for comment in comments:
            if (comment.get('user', {}).get('type') == 'Bot' and
                'Go Test Coverage Report' in comment.get('body', '')):
                return comment['id']

    except requests.exceptions.RequestException as e:
        print(f"Error fetching comments: {e}", file=sys.stderr)

    return None


def post_comment(token: str, repo: str, pr_number: int, body: str) -> bool:
    """
    Post a new comment to the PR.

    Args:
        token: GitHub token
        repo: Repository in format "owner/repo"
        pr_number: Pull request number
        body: Comment body

    Returns:
        True if successful, False otherwise
    """
    url = f"https://api.github.com/repos/{repo}/issues/{pr_number}/comments"
    headers = {
        'Authorization': f'token {token}',
        'Accept': 'application/vnd.github.v3+json'
    }
    data = {'body': body}

    try:
        response = requests.post(url, headers=headers, json=data)
        response.raise_for_status()
        print("✅ Coverage comment posted successfully")
        return True
    except requests.exceptions.RequestException as e:
        print(f"Error posting comment: {e}", file=sys.stderr)
        if hasattr(e, 'response') and e.response is not None:
            print(f"Response: {e.response.text}", file=sys.stderr)
        return False


def update_comment(token: str, repo: str, comment_id: int, body: str) -> bool:
    """
    Update an existing comment.

    Args:
        token: GitHub token
        repo: Repository in format "owner/repo"
        comment_id: Comment ID to update
        body: New comment body

    Returns:
        True if successful, False otherwise
    """
    url = f"https://api.github.com/repos/{repo}/issues/comments/{comment_id}"
    headers = {
        'Authorization': f'token {token}',
        'Accept': 'application/vnd.github.v3+json'
    }
    data = {'body': body}

    try:
        response = requests.patch(url, headers=headers, json=data)
        response.raise_for_status()
        print("✅ Coverage comment updated successfully")
        return True
    except requests.exceptions.RequestException as e:
        print(f"Error updating comment: {e}", file=sys.stderr)
        if hasattr(e, 'response') and e.response is not None:
            print(f"Response: {e.response.text}", file=sys.stderr)
        return False


def is_fork_pr(event_path: str) -> bool:
    """
    Check if the PR is from a fork.

    Args:
        event_path: Path to GitHub event JSON file

    Returns:
        True if fork PR, False otherwise
    """
    try:
        with open(event_path, 'r') as f:
            event = json.load(f)

        pr = event.get('pull_request', {})
        head_repo = pr.get('head', {}).get('repo', {}).get('full_name')
        base_repo = pr.get('base', {}).get('repo', {}).get('full_name')

        return head_repo != base_repo
    except (FileNotFoundError, json.JSONDecodeError, KeyError) as e:
        print(f"Warning: Could not determine if fork PR: {e}", file=sys.stderr)
        return False


def main():
    """Main entry point."""
    # Get environment variables
    token = os.environ.get('GITHUB_TOKEN')
    repo = os.environ.get('GITHUB_REPOSITORY')
    event_path = os.environ.get('GITHUB_EVENT_PATH', '')

    # Get arguments
    if len(sys.argv) < 6:
        print("Usage: comment_pr.py <pr_number> <coverage> <emoji> <status> <badge_color> [coverage_report_path]",
              file=sys.stderr)
        sys.exit(1)

    pr_number = int(sys.argv[1])
    coverage = sys.argv[2]
    emoji = sys.argv[3]
    status = sys.argv[4]
    badge_color = sys.argv[5]
    coverage_report_path = sys.argv[6] if len(sys.argv) > 6 else 'coverage_report.md'

    # Validate environment
    if not token:
        print("Error: GITHUB_TOKEN environment variable not set", file=sys.stderr)
        sys.exit(1)

    if not repo:
        print("Error: GITHUB_REPOSITORY environment variable not set", file=sys.stderr)
        sys.exit(1)

    # Check if fork PR
    if event_path and is_fork_pr(event_path):
        print("ℹ️  Fork PR detected - skipping comment (no write permissions)")
        sys.exit(0)

    # Generate comment body
    comment_body = generate_comment_body(coverage, emoji, status, badge_color, coverage_report_path)

    # Check for existing comment
    existing_comment_id = find_existing_comment(token, repo, pr_number)

    # Post or update comment
    if existing_comment_id:
        print(f"Found existing comment (ID: {existing_comment_id}), updating...")
        success = update_comment(token, repo, existing_comment_id, comment_body)
    else:
        print("No existing comment found, creating new one...")
        success = post_comment(token, repo, pr_number, comment_body)

    sys.exit(0 if success else 1)


if __name__ == '__main__':
    main()
