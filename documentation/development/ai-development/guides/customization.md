# Cursor IDE Customization Guide

## Overview

This guide provides comprehensive instructions for customizing Cursor IDE to match your preferences and workflow.

## Table of Contents

1. [Settings](#settings)
2. [Themes](#themes)
3. [Extensions](#extensions)
4. [Keyboard Shortcuts](#keyboard-shortcuts)
5. [AI Configuration](#ai-configuration)
6. [Editor Behavior](#editor-behavior)
7. [Terminal Customization](#terminal-customization)
8. [Workspace Settings](#workspace-settings)

## Settings

### Accessing Settings

1. **Command Palette**

   ```
   Cmd/Ctrl + Shift + P
   Type "Preferences: Open Settings"
   ```

2. **Keyboard Shortcut**

   ```
   Cmd/Ctrl + ,
   ```

3. **Menu Bar**
   ```
   Code > Preferences > Settings
   ```

### Settings Categories

1. **Editor**

   ```json
   {
     "editor.fontSize": 14,
     "editor.fontFamily": "Fira Code",
     "editor.fontLigatures": true,
     "editor.lineHeight": 1.5,
     "editor.letterSpacing": 0.5,
     "editor.wordWrap": "on",
     "editor.tabSize": 2,
     "editor.insertSpaces": true,
     "editor.detectIndentation": true,
     "editor.renderWhitespace": "all",
     "editor.renderControlCharacters": true,
     "editor.minimap.enabled": true,
     "editor.minimap.renderCharacters": false,
     "editor.minimap.maxColumn": 80,
     "editor.minimap.showSlider": "always"
   }
   ```

2. **Workbench**

   ```json
   {
     "workbench.colorTheme": "One Dark Pro",
     "workbench.iconTheme": "material-icon-theme",
     "workbench.startupEditor": "newUntitledFile",
     "workbench.editor.enablePreview": true,
     "workbench.editor.enablePreviewFromQuickOpen": true,
     "workbench.editor.highlightModifiedTabs": true,
     "workbench.editor.showTabs": true,
     "workbench.editor.tabSizing": "shrink",
     "workbench.editor.wrapTabs": false,
     "workbench.sideBar.location": "left",
     "workbench.sideBar.visible": true,
     "workbench.statusBar.visible": true,
     "workbench.activityBar.visible": true
   }
   ```

3. **Files**
   ```json
   {
     "files.autoSave": "afterDelay",
     "files.autoSaveDelay": 1000,
     "files.autoSaveOnFocusChange": true,
     "files.autoSaveOnWindowChange": true,
     "files.hotExit": "onExitAndWindowClose",
     "files.restoreUndoStack": true,
     "files.trimTrailingWhitespace": true,
     "files.insertFinalNewline": true,
     "files.trimFinalNewlines": true
   }
   ```

## Themes

### Built-in Themes

1. **Light Themes**

   - Default Light
   - Light+ (default light)
   - Solarized Light
   - GitHub Light

2. **Dark Themes**
   - Default Dark
   - Dark+ (default dark)
   - Monokai
   - Solarized Dark
   - One Dark Pro
   - GitHub Dark

### Custom Themes

1. **Install from Marketplace**

   ```
   Cmd/Ctrl + Shift + X
   Search for "theme"
   ```

2. **Create Custom Theme**
   ```json
   {
     "name": "My Custom Theme",
     "type": "dark",
     "colors": {
       "editor.background": "#1E1E1E",
       "editor.foreground": "#D4D4D4",
       "activityBar.background": "#333333",
       "activityBar.foreground": "#FFFFFF",
       "sideBar.background": "#252526",
       "sideBar.foreground": "#CCCCCC"
     }
   }
   ```

## Extensions

### Recommended Extensions

1. **Productivity**

   - GitLens
   - Error Lens
   - Prettier
   - ESLint
   - Docker

2. **Language Support**

   - Python
   - JavaScript/TypeScript
   - Go
   - Rust
   - Java

3. **Themes**
   - One Dark Pro
   - Material Icon Theme
   - Dracula Official
   - Night Owl

### Extension Settings

1. **GitLens**

   ```json
   {
     "gitlens.codeLens.enabled": true,
     "gitlens.currentLine.enabled": true,
     "gitlens.hovers.enabled": true,
     "gitlens.statusBar.enabled": true
   }
   ```

2. **Prettier**

   ```json
   {
     "prettier.singleQuote": true,
     "prettier.trailingComma": "es5",
     "prettier.printWidth": 80,
     "prettier.tabWidth": 2,
     "prettier.useTabs": false
   }
   ```

3. **ESLint**
   ```json
   {
     "eslint.enable": true,
     "eslint.run": "onType",
     "eslint.validate": [
       "javascript",
       "javascriptreact",
       "typescript",
       "typescriptreact"
     ]
   }
   ```

## Keyboard Shortcuts

### Customizing Shortcuts

1. **Access Keyboard Shortcuts**

   ```
   Cmd/Ctrl + K Cmd/Ctrl + S
   ```

2. **Add Custom Shortcut**

   ```json
   {
     "key": "cmd+shift+r",
     "command": "workbench.action.quickOpen",
     "args": ">"
   }
   ```

3. **Keybindings.json**
   ```json
   [
     {
       "key": "cmd+shift+r",
       "command": "workbench.action.quickOpen",
       "args": ">"
     },
     {
       "key": "cmd+shift+f",
       "command": "workbench.action.findInFiles"
     }
   ]
   ```

## AI Configuration

### AI Settings

1. **Basic Settings**

   ```json
   {
     "cursor.ai.enabled": true,
     "cursor.ai.autoComplete": true,
     "cursor.ai.suggestions": true,
     "cursor.ai.chat.enabled": true
   }
   ```

2. **Code Generation**

   ```json
   {
     "cursor.ai.codeGeneration.enabled": true,
     "cursor.ai.codeGeneration.autoApply": false,
     "cursor.ai.codeGeneration.promptTemplate": "Generate code for: {prompt}"
   }
   ```

3. **Code Review**
   ```json
   {
     "cursor.ai.codeReview.enabled": true,
     "cursor.ai.codeReview.autoReview": false,
     "cursor.ai.codeReview.reviewTemplate": "Review code for: {file}"
   }
   ```

## Editor Behavior

### Editor Settings

1. **Basic Behavior**

   ```json
   {
     "editor.wordWrap": "on",
     "editor.lineNumbers": "on",
     "editor.cursorStyle": "line",
     "editor.cursorBlinking": "blink",
     "editor.cursorSmoothCaretAnimation": true,
     "editor.cursorWidth": 2
   }
   ```

2. **Selection**

   ```json
   {
     "editor.selectionHighlight": true,
     "editor.occurrencesHighlight": true,
     "editor.semanticHighlighting.enabled": true,
     "editor.bracketPairColorization.enabled": true
   }
   ```

3. **Folding**
   ```json
   {
     "editor.folding": true,
     "editor.foldingStrategy": "auto",
     "editor.foldingHighlight": true,
     "editor.showFoldingControls": "always"
   }
   ```

## Terminal Customization

### Terminal Settings

1. **Appearance**

   ```json
   {
     "terminal.integrated.fontSize": 14,
     "terminal.integrated.fontFamily": "Fira Code",
     "terminal.integrated.fontLigatures": true,
     "terminal.integrated.lineHeight": 1.5
   }
   ```

2. **Behavior**

   ```json
   {
     "terminal.integrated.copyOnSelection": true,
     "terminal.integrated.cursorBlinking": true,
     "terminal.integrated.cursorStyle": "line",
     "terminal.integrated.scrollback": 1000
   }
   ```

3. **Shell**
   ```json
   {
     "terminal.integrated.shell.osx": "/bin/zsh",
     "terminal.integrated.shell.linux": "/bin/bash",
     "terminal.integrated.shell.windows": "C:\\Windows\\System32\\cmd.exe"
   }
   ```

## Workspace Settings

### Workspace Configuration

1. **Basic Settings**

   ```json
   {
     "files.exclude": {
       "**/.git": true,
       "**/.svn": true,
       "**/.hg": true,
       "**/CVS": true,
       "**/.DS_Store": true,
       "**/Thumbs.db": true
     },
     "search.exclude": {
       "**/node_modules": true,
       "**/bower_components": true,
       "**/*.code-search": true
     }
   }
   ```

2. **Language Settings**

   ```json
   {
     "[javascript]": {
       "editor.defaultFormatter": "esbenp.prettier-vscode",
       "editor.formatOnSave": true
     },
     "[typescript]": {
       "editor.defaultFormatter": "esbenp.prettier-vscode",
       "editor.formatOnSave": true
     },
     "[python]": {
       "editor.defaultFormatter": "ms-python.python",
       "editor.formatOnSave": true
     }
   }
   ```

3. **Project Settings**
   ```json
   {
     "python.linting.enabled": true,
     "python.linting.pylintEnabled": true,
     "python.formatting.provider": "black",
     "python.testing.pytestEnabled": true
   }
   ```

## Resources

### Documentation

- [Cursor Documentation](https://cursor.sh/docs)
- [Settings Reference](https://cursor.sh/docs/settings)
- [Themes Guide](https://cursor.sh/docs/themes)

### Community

- [Discord Server](https://discord.gg/cursor)
- [GitHub Issues](https://github.com/getcursor/cursor/issues)
- [Twitter Updates](https://twitter.com/cursor_ai)

## Contributing

Feel free to contribute to this guide by:

1. Adding new settings
2. Improving existing content
3. Fixing errors
4. Adding examples

## License

This guide is licensed under the MIT License.
