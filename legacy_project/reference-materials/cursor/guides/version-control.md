# Version Control in Cursor IDE

## Overview

This guide provides comprehensive instructions for using version control features in Cursor IDE, focusing on Git integration and best practices.

## Table of Contents

1. [Getting Started](#getting-started)
2. [Basic Git Operations](#basic-git-operations)
3. [Advanced Git Features](#advanced-git-features)
4. [Git Integration](#git-integration)
5. [Best Practices](#best-practices)
6. [Troubleshooting](#troubleshooting)

## Getting Started

### Git Configuration

1. **Global Configuration**

   ```bash
   git config --global user.name "Your Name"
   git config --global user.email "your.email@example.com"
   git config --global core.editor "cursor"
   ```

2. **Repository Configuration**

   ```bash
   git config user.name "Your Name"
   git config user.email "your.email@example.com"
   git config core.editor "cursor"
   ```

3. **Git Settings**
   ```json
   {
     "git.enabled": true,
     "git.autofetch": true,
     "git.confirmSync": true,
     "git.enableSmartCommit": true
   }
   ```

### Initializing a Repository

1. **New Repository**

   ```bash
   git init
   git add .
   git commit -m "Initial commit"
   ```

2. **Clone Repository**

   ```bash
   git clone <repository-url>
   ```

3. **Remote Setup**
   ```bash
   git remote add origin <repository-url>
   git push -u origin main
   ```

## Basic Git Operations

### Committing Changes

1. **Stage Changes**

   ```
   Cmd/Ctrl + Shift + G
   Click + next to files
   ```

2. **Commit Changes**

   ```
   Cmd/Ctrl + Enter
   Type commit message
   ```

3. **Commit Settings**
   ```json
   {
     "git.enableSmartCommit": true,
     "git.suggestSmartCommit": true,
     "git.confirmSync": true
   }
   ```

### Branching

1. **Create Branch**

   ```
   Cmd/Ctrl + Shift + P
   Type "Git: Create Branch"
   ```

2. **Switch Branch**

   ```
   Cmd/Ctrl + Shift + P
   Type "Git: Checkout to"
   ```

3. **Branch Settings**
   ```json
   {
     "git.branchSortOrder": "committerdate",
     "git.branchProtection": ["main", "master"],
     "git.branchValidationRegex": ""
   }
   ```

### Merging

1. **Merge Branch**

   ```
   Cmd/Ctrl + Shift + P
   Type "Git: Merge Branch"
   ```

2. **Resolve Conflicts**

   ```
   Cmd/Ctrl + Shift + P
   Type "Git: Resolve Conflicts"
   ```

3. **Merge Settings**
   ```json
   {
     "git.mergeEditor": true,
     "git.confirmSync": true,
     "git.enableSmartCommit": true
   }
   ```

## Advanced Git Features

### Stashing

1. **Stash Changes**

   ```
   Cmd/Ctrl + Shift + P
   Type "Git: Stash"
   ```

2. **Apply Stash**

   ```
   Cmd/Ctrl + Shift + P
   Type "Git: Pop Stash"
   ```

3. **Stash Settings**
   ```json
   {
     "git.stashOnPull": true,
     "git.stashOnSwitch": true
   }
   ```

### Rebasing

1. **Start Rebase**

   ```
   Cmd/Ctrl + Shift + P
   Type "Git: Rebase"
   ```

2. **Interactive Rebase**

   ```
   Cmd/Ctrl + Shift + P
   Type "Git: Rebase Interactive"
   ```

3. **Rebase Settings**
   ```json
   {
     "git.rebaseWhenSync": true,
     "git.confirmSync": true
   }
   ```

### Cherry Picking

1. **Cherry Pick**

   ```
   Cmd/Ctrl + Shift + P
   Type "Git: Cherry Pick"
   ```

2. **Resolve Conflicts**

   ```
   Cmd/Ctrl + Shift + P
   Type "Git: Resolve Conflicts"
   ```

3. **Cherry Pick Settings**
   ```json
   {
     "git.confirmSync": true,
     "git.enableSmartCommit": true
   }
   ```

## Git Integration

### GitLens Integration

1. **Installation**

   ```
   Cmd/Ctrl + Shift + X
   Search for "GitLens"
   ```

2. **Features**

   - Blame annotations
   - File history
   - Branch comparison
   - Commit search

3. **Settings**
   ```json
   {
     "gitlens.codeLens.enabled": true,
     "gitlens.currentLine.enabled": true,
     "gitlens.hovers.enabled": true,
     "gitlens.statusBar.enabled": true
   }
   ```

### GitHub Integration

1. **Authentication**

   ```
   Cmd/Ctrl + Shift + P
   Type "GitHub: Sign in"
   ```

2. **Features**

   - Pull requests
   - Issues
   - Actions
   - Codespaces

3. **Settings**
   ```json
   {
     "github.hosts": [
       {
         "host": "github.com",
         "username": "your-username",
         "token": "your-token"
       }
     ]
   }
   ```

### Git Graph

1. **Installation**

   ```
   Cmd/Ctrl + Shift + X
   Search for "Git Graph"
   ```

2. **Features**

   - Visual commit graph
   - Branch visualization
   - Commit details
   - Quick actions

3. **Settings**
   ```json
   {
     "git-graph.repository.commits.showSignatureStatus": true,
     "git-graph.repository.commits.showRemoteHeads": true,
     "git-graph.repository.commits.showStashes": true
   }
   ```

## Best Practices

### Commit Messages

1. **Format**

   ```
   <type>(<scope>): <subject>

   <body>

   <footer>
   ```

2. **Types**

   - feat: New feature
   - fix: Bug fix
   - docs: Documentation
   - style: Formatting
   - refactor: Code restructuring
   - test: Testing
   - chore: Maintenance

3. **Settings**
   ```json
   {
     "git.commitTemplate": "type(scope): subject\n\nbody\n\nfooter",
     "git.enableSmartCommit": true,
     "git.suggestSmartCommit": true
   }
   ```

### Branching Strategy

1. **Main Branches**

   - main/master: Production
   - develop: Development
   - feature/\*: Features
   - bugfix/\*: Bug fixes
   - release/\*: Releases
   - hotfix/\*: Hot fixes

2. **Workflow**

   - Create feature branch
   - Develop feature
   - Create pull request
   - Review and merge
   - Delete branch

3. **Settings**
   ```json
   {
     "git.branchProtection": ["main", "master"],
     "git.branchValidationRegex": "",
     "git.branchSortOrder": "committerdate"
   }
   ```

### Code Review

1. **Process**

   - Create pull request
   - Review changes
   - Add comments
   - Approve changes
   - Merge changes

2. **Settings**
   ```json
   {
     "github.pullRequests.ignoreDraftWithPendingReview": true,
     "github.pullRequests.ignoreDraftWithApprovedReview": true,
     "github.pullRequests.ignoreDraftWithChangesRequestedReview": true
   }
   ```

## Troubleshooting

### Common Issues

1. **Authentication**

   - Check credentials
   - Verify tokens
   - Update permissions
   - Clear cache

2. **Merge Conflicts**

   - Review changes
   - Resolve conflicts
   - Test changes
   - Commit resolution

3. **Performance**
   - Check repository size
   - Clean up history
   - Update Git
   - Clear cache

### Solutions

1. **Git Issues**

   - Check configuration
   - Verify permissions
   - Update Git
   - Clear cache

2. **Extension Issues**

   - Update extensions
   - Check compatibility
   - Review settings
   - Reinstall if needed

3. **System Issues**
   - Check resources
   - Update system
   - Clear cache
   - Restart IDE

## Resources

### Documentation

- [Git Documentation](https://git-scm.com/doc)
- [GitHub Documentation](https://docs.github.com)
- [GitLens Documentation](https://gitlens.amod.io)

### Community

- [GitHub Community](https://github.community)
- [Stack Overflow](https://stackoverflow.com/questions/tagged/git)
- [GitHub Issues](https://github.com/getcursor/cursor/issues)

## Contributing

Feel free to contribute to this guide by:

1. Adding new Git features
2. Improving existing content
3. Fixing errors
4. Adding examples

## License

This guide is licensed under the MIT License.
