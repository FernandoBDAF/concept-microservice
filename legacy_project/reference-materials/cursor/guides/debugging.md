# Debugging in Cursor IDE

## Overview

This guide provides comprehensive instructions for debugging applications in Cursor IDE, covering various languages and debugging scenarios.

## Table of Contents

1. [Getting Started](#getting-started)
2. [Debugging Features](#debugging-features)
3. [Language-Specific Debugging](#language-specific-debugging)
4. [Advanced Debugging](#advanced-debugging)
5. [Troubleshooting](#troubleshooting)

## Getting Started

### Launching the Debugger

1. **Command Palette**

   ```
   Cmd/Ctrl + Shift + P
   Type "Debug: Start Debugging"
   ```

2. **Keyboard Shortcut**

   ```
   F5
   ```

3. **Menu Bar**
   ```
   Run > Start Debugging
   ```

### Debug Configuration

1. **Basic Configuration**

   ```json
   {
     "version": "0.2.0",
     "configurations": [
       {
         "name": "Python: Current File",
         "type": "python",
         "request": "launch",
         "program": "${file}",
         "console": "integratedTerminal"
       }
     ]
   }
   ```

2. **Node.js Configuration**

   ```json
   {
     "version": "0.2.0",
     "configurations": [
       {
         "name": "Node.js: Current File",
         "type": "node",
         "request": "launch",
         "program": "${file}",
         "console": "integratedTerminal"
       }
     ]
   }
   ```

3. **Go Configuration**
   ```json
   {
     "version": "0.2.0",
     "configurations": [
       {
         "name": "Go: Current File",
         "type": "go",
         "request": "launch",
         "mode": "auto",
         "program": "${file}"
       }
     ]
   }
   ```

## Debugging Features

### Breakpoints

1. **Setting Breakpoints**

   - Click in the gutter (left margin)
   - Use F9 keyboard shortcut
   - Use command palette: "Debug: Toggle Breakpoint"

2. **Breakpoint Types**

   - Regular breakpoints
   - Conditional breakpoints
   - Log points
   - Function breakpoints

3. **Breakpoint Management**
   ```json
   {
     "debug.allowBreakpointsEverywhere": true,
     "debug.breakOnExceptions": true,
     "debug.breakOnUncaughtExceptions": true
   }
   ```

### Debug Console

1. **Accessing Console**

   ```
   Cmd/Ctrl + Shift + Y
   ```

2. **Console Features**

   - Evaluate expressions
   - View variable values
   - Execute commands
   - View debug output

3. **Console Settings**
   ```json
   {
     "debug.console.fontSize": 14,
     "debug.console.fontFamily": "Fira Code",
     "debug.console.lineHeight": 1.5
   }
   ```

### Watch Variables

1. **Adding Watch**

   - Right-click variable
   - Select "Add to Watch"
   - Use command palette: "Debug: Add to Watch"

2. **Watch Expressions**

   ```javascript
   // Example watch expressions
   this.state;
   this.props;
   this.context;
   ```

3. **Watch Settings**
   ```json
   {
     "debug.watch.useNaturalSort": true,
     "debug.watch.maxStringLength": 100
   }
   ```

## Language-Specific Debugging

### Python Debugging

1. **Configuration**

   ```json
   {
     "version": "0.2.0",
     "configurations": [
       {
         "name": "Python: Current File",
         "type": "python",
         "request": "launch",
         "program": "${file}",
         "console": "integratedTerminal",
         "justMyCode": true
       }
     ]
   }
   ```

2. **Debugging Features**

   - Step through code
   - Inspect variables
   - Evaluate expressions
   - View call stack

3. **Python Settings**
   ```json
   {
     "python.debugpy.useVenv": true,
     "python.debugpy.port": 5678,
     "python.debugpy.host": "localhost"
   }
   ```

### JavaScript/TypeScript Debugging

1. **Configuration**

   ```json
   {
     "version": "0.2.0",
     "configurations": [
       {
         "name": "Node.js: Current File",
         "type": "node",
         "request": "launch",
         "program": "${file}",
         "console": "integratedTerminal",
         "skipFiles": ["<node_internals>/**"]
       }
     ]
   }
   ```

2. **Debugging Features**

   - Source maps support
   - Async debugging
   - Node.js debugging
   - Browser debugging

3. **JavaScript Settings**
   ```json
   {
     "debug.javascript.autoAttachFilter": "smart",
     "debug.javascript.terminalOptions": {
       "skipFiles": ["<node_internals>/**"]
     }
   }
   ```

### Go Debugging

1. **Configuration**

   ```json
   {
     "version": "0.2.0",
     "configurations": [
       {
         "name": "Go: Current File",
         "type": "go",
         "request": "launch",
         "mode": "auto",
         "program": "${file}",
         "showLog": true
       }
     ]
   }
   ```

2. **Debugging Features**

   - Goroutine debugging
   - Delve integration
   - Test debugging
   - Remote debugging

3. **Go Settings**
   ```json
   {
     "go.delveConfig": {
       "dlvLoadConfig": {
         "followPointers": true,
         "maxVariableRecurse": 1,
         "maxStringLen": 512,
         "maxArrayValues": 64,
         "maxStructFields": -1
       }
     }
   }
   ```

## Advanced Debugging

### Remote Debugging

1. **Configuration**

   ```json
   {
     "version": "0.2.0",
     "configurations": [
       {
         "name": "Remote Debug",
         "type": "node",
         "request": "attach",
         "address": "localhost",
         "port": 9229,
         "localRoot": "${workspaceFolder}",
         "remoteRoot": "/app"
       }
     ]
   }
   ```

2. **Remote Settings**
   ```json
   {
     "debug.allowBreakpointsEverywhere": true,
     "debug.console.closeOnEnd": true,
     "debug.inlineValues": true
   }
   ```

### Multi-Process Debugging

1. **Configuration**

   ```json
   {
     "version": "0.2.0",
     "configurations": [
       {
         "name": "Multi-Process Debug",
         "type": "node",
         "request": "launch",
         "program": "${workspaceFolder}/server.js",
         "processId": "${command:pickProcess}"
       }
     ]
   }
   ```

2. **Process Settings**
   ```json
   {
     "debug.javascript.autoAttachFilter": "smart",
     "debug.javascript.terminalOptions": {
       "skipFiles": ["<node_internals>/**"]
     }
   }
   ```

### Performance Profiling

1. **CPU Profiling**

   ```json
   {
     "version": "0.2.0",
     "configurations": [
       {
         "name": "CPU Profile",
         "type": "node",
         "request": "launch",
         "program": "${file}",
         "runtimeArgs": ["--prof"]
       }
     ]
   }
   ```

2. **Memory Profiling**
   ```json
   {
     "version": "0.2.0",
     "configurations": [
       {
         "name": "Memory Profile",
         "type": "node",
         "request": "launch",
         "program": "${file}",
         "runtimeArgs": ["--inspect-brk"]
       }
     ]
   }
   ```

## Troubleshooting

### Common Issues

1. **Debugger Not Starting**

   - Check configuration
   - Verify file paths
   - Check permissions
   - Review logs

2. **Breakpoints Not Hitting**

   - Check source maps
   - Verify file paths
   - Check debugger settings
   - Review configuration

3. **Performance Issues**
   - Check debugger settings
   - Review configuration
   - Check system resources
   - Update extensions

### Solutions

1. **Configuration Issues**

   - Review launch.json
   - Check file paths
   - Verify settings
   - Update configuration

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

- [Debugging Guide](https://cursor.sh/docs/debugging)
- [Configuration Reference](https://cursor.sh/docs/configuration)
- [Troubleshooting Guide](https://cursor.sh/docs/troubleshooting)

### Community

- [Discord Server](https://discord.gg/cursor)
- [GitHub Issues](https://github.com/getcursor/cursor/issues)
- [Stack Overflow](https://stackoverflow.com/questions/tagged/cursor-ide)

## Contributing

Feel free to contribute to this guide by:

1. Adding new debugging techniques
2. Improving existing content
3. Fixing errors
4. Adding examples

## License

This guide is licensed under the MIT License.
