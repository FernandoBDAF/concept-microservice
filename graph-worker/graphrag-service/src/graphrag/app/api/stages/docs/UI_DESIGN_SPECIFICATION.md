# Stages API - UI Design Specification

**Version:** 1.0  
**Created:** December 9, 2025  
**Reference:** [STAGES_API_TECHNICAL_FOUNDATION.md](./STAGES_API_TECHNICAL_FOUNDATION.md) Section 9

---

## Table of Contents

1. [Overview](#1-overview)
2. [Technology Stack](#2-technology-stack)
3. [Application Architecture](#3-application-architecture)
4. [Component Hierarchy](#4-component-hierarchy)
5. [State Management](#5-state-management)
6. [API Integration](#6-api-integration)
7. [Component Specifications](#7-component-specifications)
8. [Form Generation System](#8-form-generation-system)
9. [Validation & Error Handling](#9-validation--error-handling)
10. [Execution & Status Monitoring](#10-execution--status-monitoring)
11. [Styling & UX Guidelines](#11-styling--ux-guidelines)
12. [Implementation Phases](#12-implementation-phases)

---

## 1. Overview

### 1.1 Purpose

Create a minimalist, functional UI for configuring and executing GraphRAG and Ingestion pipelines. The UI should:

- Allow users to select a pipeline type and stages
- Generate dynamic configuration forms based on API schema
- Validate configurations before execution
- Execute pipelines and monitor status

### 1.2 Design Philosophy

- **Minimalist**: Clean interface with only essential elements
- **Functional**: Every element serves a clear purpose
- **Dynamic**: Forms generated from API schema, not hardcoded
- **Responsive**: Works on desktop and tablet screens

### 1.3 User Flow Summary

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Select    │ → │   Select    │ → │  Configure  │ → │   Execute   │
│  Pipeline   │    │   Stages    │    │   Stages    │    │  Pipeline   │
└─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘
       │                  │                  │                  │
       ▼                  ▼                  ▼                  ▼
 GET /stages      Show stage list     GET /stages/     POST /pipelines/
                  with dependencies   {name}/config      execute
```

---

## 2. Technology Stack

### 2.1 Recommended Stack

| Layer | Technology | Rationale |
|-------|------------|-----------|
| **Framework** | React 18+ or Vue 3 | Component-based, excellent form handling |
| **Styling** | Tailwind CSS | Rapid prototyping, utility-first |
| **State** | React Context or Zustand | Simple state management |
| **HTTP Client** | Fetch API or Axios | API communication |
| **Forms** | React Hook Form | Dynamic form handling with validation |

### 2.2 Alternative: Vanilla JS + HTML

For maximum simplicity, the UI could be built with:
- Vanilla JavaScript (ES6+)
- HTML5 with semantic elements
- CSS3 with CSS Variables
- Native Fetch API

### 2.3 File Structure

```
ui/
├── index.html              # Main HTML entry point
├── styles/
│   └── main.css            # All styles
├── js/
│   ├── app.js              # Main application logic
│   ├── api.js              # API client
│   ├── components/
│   │   ├── PipelineSelector.js
│   │   ├── StageSelector.js
│   │   ├── ConfigForm.js
│   │   ├── FieldRenderer.js
│   │   ├── ExecutionPanel.js
│   │   └── StatusMonitor.js
│   └── utils/
│       ├── formGenerator.js
│       └── validation.js
└── assets/
    └── icons/
```

---

## 3. Application Architecture

### 3.1 Page Layout

```
┌────────────────────────────────────────────────────────────────────┐
│                           HEADER                                    │
│  Stage Configuration                                    [Help] [?]  │
├────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │                   PIPELINE SELECTOR                          │  │
│  │  ◉ GraphRAG Pipeline    ○ Ingestion Pipeline                │  │
│  │  Build knowledge graph from processed chunks...              │  │
│  └─────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │                    STAGE SELECTOR                            │  │
│  │  ☑ Graph Extraction       ☐ Graph Construction              │  │
│  │  ☑ Entity Resolution      ☐ Community Detection             │  │
│  │                                                              │  │
│  │  ⚠ Dependency: Entity Resolution requires Graph Extraction  │  │
│  └─────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │                 CONFIGURATION PANELS                         │  │
│  │                                                              │  │
│  │  ┌─ Graph Extraction ─────────────────────────────────────┐ │  │
│  │  │  ▼ LLM Settings                                        │ │  │
│  │  │    Model: [gpt-4o-mini ▼]  Temperature: [====●===] 0.1 │ │  │
│  │  │                                                        │ │  │
│  │  │  ▼ Processing                                          │ │  │
│  │  │    Concurrency: [50]  Max Documents: [___]             │ │  │
│  │  └────────────────────────────────────────────────────────┘ │  │
│  │                                                              │  │
│  │  ┌─ Entity Resolution ────────────────────────────────────┐ │  │
│  │  │  ... configuration fields ...                          │ │  │
│  │  └────────────────────────────────────────────────────────┘ │  │
│  └─────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │                   EXECUTION PANEL                            │  │
│  │                                                              │  │
│  │  [ Validate Configuration ]     [ Execute Pipeline ]        │  │
│  │                                                              │  │
│  │  ┌─ Validation Results ─────────────────────────────────┐   │  │
│  │  │  ✓ Configuration valid                               │   │  │
│  │  │  ⚠ Warning: Stage X depends on Y (auto-included)     │   │  │
│  │  └──────────────────────────────────────────────────────┘   │  │
│  └─────────────────────────────────────────────────────────────┘  │
│                                                                     │
├────────────────────────────────────────────────────────────────────┤
│                           FOOTER                                    │
│  API Status: ● Connected                              v1.0.0       │
└────────────────────────────────────────────────────────────────────┘
```

### 3.2 Responsive Breakpoints

| Breakpoint | Width | Layout |
|------------|-------|--------|
| Desktop | ≥1024px | Full layout, side-by-side panels |
| Tablet | 768-1023px | Stacked panels, full-width forms |
| Mobile | <768px | Single column, accordion sections |

---

## 4. Component Hierarchy

### 4.1 Component Tree

```
App
├── Header
│   └── HelpButton
├── Main
│   ├── PipelineSelector
│   │   ├── RadioOption (GraphRAG)
│   │   └── RadioOption (Ingestion)
│   ├── StageSelector
│   │   ├── StageCheckbox (per stage)
│   │   └── DependencyWarning
│   ├── ConfigurationPanels
│   │   └── StageConfigPanel (per selected stage)
│   │       ├── CategorySection (per category)
│   │       │   └── ConfigField (per field)
│   │       └── CollapseToggle
│   └── ExecutionPanel
│       ├── ValidateButton
│       ├── ExecuteButton
│       ├── ValidationResults
│       └── StatusMonitor (appears after execution)
└── Footer
    └── ApiStatus
```

### 4.2 Component Responsibilities

| Component | Responsibility |
|-----------|----------------|
| `PipelineSelector` | Select pipeline type, display description |
| `StageSelector` | Multi-select stages, show dependencies |
| `StageConfigPanel` | Render configuration form for one stage |
| `CategorySection` | Group fields by category |
| `ConfigField` | Render individual field based on type |
| `ExecutionPanel` | Validate, execute, show results |
| `StatusMonitor` | Poll and display execution status |

---

## 5. State Management

### 5.1 Application State Schema

```typescript
interface AppState {
  // Pipeline selection
  selectedPipeline: 'graphrag' | 'ingestion' | null;
  
  // Stage selection
  selectedStages: string[];
  
  // Stage configurations (key = stage name)
  stageConfigs: {
    [stageName: string]: {
      [fieldName: string]: any;
    };
  };
  
  // API data (cached)
  availableStages: StagesResponse | null;
  stageSchemas: {
    [stageName: string]: StageConfigSchema;
  };
  stageDefaults: {
    [stageName: string]: StageDefaults;
  };
  
  // Validation state
  validationResult: ValidationResult | null;
  validationLoading: boolean;
  
  // Execution state
  executionResult: ExecutionResult | null;
  executionLoading: boolean;
  currentPipelineId: string | null;
  pipelineStatus: PipelineStatus | null;
  
  // UI state
  expandedPanels: string[];
  errors: string[];
}
```

### 5.2 State Actions

```typescript
// Pipeline actions
selectPipeline(pipeline: 'graphrag' | 'ingestion'): void;

// Stage actions
toggleStage(stageName: string): void;
setStageConfig(stageName: string, fieldName: string, value: any): void;
resetStageConfig(stageName: string): void;

// API actions
fetchStages(): Promise<void>;
fetchStageConfig(stageName: string): Promise<void>;
fetchStageDefaults(stageName: string): Promise<void>;

// Execution actions
validateConfig(): Promise<void>;
executePipeline(): Promise<void>;
pollPipelineStatus(pipelineId: string): Promise<void>;
cancelPipeline(): Promise<void>;
```

### 5.3 State Flow Diagram

```
User Action          State Update              Side Effect
───────────────────────────────────────────────────────────────
Select Pipeline  →  selectedPipeline = X   →  Clear stages
                    selectedStages = []       Fetch stage list
                    
Toggle Stage     →  selectedStages.toggle  →  Fetch schema if needed
                    Check dependencies         Auto-add dependencies
                    
Change Field     →  stageConfigs[stage]    →  Clear validation
                    [field] = value           Mark form dirty
                    
Click Validate   →  validationLoading      →  POST /pipelines/validate
                    = true                    Update validationResult
                    
Click Execute    →  executionLoading       →  POST /pipelines/execute
                    = true                    Start polling status
```

---

## 6. API Integration

### 6.1 API Client Module

```javascript
// api.js

const BASE_URL = 'http://localhost:8080/api/v1';

export const api = {
  // Stage discovery
  async listStages() {
    const response = await fetch(`${BASE_URL}/stages`);
    return response.json();
  },
  
  async listPipelineStages(pipeline) {
    const response = await fetch(`${BASE_URL}/stages/${pipeline}`);
    return response.json();
  },
  
  async getStageConfig(stageName) {
    const response = await fetch(`${BASE_URL}/stages/${stageName}/config`);
    return response.json();
  },
  
  async getStageDefaults(stageName) {
    const response = await fetch(`${BASE_URL}/stages/${stageName}/defaults`);
    return response.json();
  },
  
  // Validation
  async validatePipeline(pipeline, stages, config) {
    const response = await fetch(`${BASE_URL}/pipelines/validate`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ pipeline, stages, config })
    });
    return response.json();
  },
  
  // Execution
  async executePipeline(pipeline, stages, config, metadata = {}) {
    const response = await fetch(`${BASE_URL}/pipelines/execute`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ pipeline, stages, config, metadata })
    });
    return response.json();
  },
  
  async getPipelineStatus(pipelineId) {
    const response = await fetch(`${BASE_URL}/pipelines/${pipelineId}/status`);
    return response.json();
  },
  
  async cancelPipeline(pipelineId) {
    const response = await fetch(`${BASE_URL}/pipelines/${pipelineId}/cancel`, {
      method: 'POST'
    });
    return response.json();
  }
};
```

### 6.2 API Response Types

```typescript
// Stage list response
interface StagesResponse {
  pipelines: {
    [name: string]: {
      name: string;
      description: string;
      stages: string[];
      stage_count: number;
    };
  };
  stages: {
    [name: string]: {
      name: string;
      display_name: string;
      description: string;
      pipeline: string;
      config_class: string;
      dependencies: string[];
      has_llm: boolean;
    };
  };
}

// Stage config schema
interface StageConfigSchema {
  stage_name: string;
  config_class: string;
  description: string;
  fields: ConfigField[];
  categories: Category[];
  field_count: number;
}

interface ConfigField {
  name: string;
  type: string;          // "string", "integer", "number", "boolean", "array"
  python_type: string;
  default: any;
  required: boolean;
  optional: boolean;
  description: string;
  category: string;
  ui_type: string;       // "text", "number", "slider", "checkbox", "select", "multiselect"
  is_inherited: boolean;
  // Optional UI hints
  min?: number;
  max?: number;
  step?: number;
  options?: string[];
  placeholder?: string;
  recommended?: any;
}

interface Category {
  name: string;
  fields: string[];
  field_count: number;
}
```

---

## 7. Component Specifications

### 7.1 PipelineSelector

**Purpose:** Allow user to select between GraphRAG and Ingestion pipelines.

**Props:**
```typescript
interface PipelineSelectorProps {
  pipelines: Pipeline[];
  selectedPipeline: string | null;
  onSelect: (pipelineName: string) => void;
}
```

**Behavior:**
- Render radio button for each pipeline
- Show pipeline name and description
- Call `onSelect` when user clicks a radio button
- Highlight selected pipeline

**UI States:**
- Default: No selection, both options enabled
- Selected: One option highlighted
- Loading: Dimmed while fetching stages

**HTML Structure:**
```html
<div class="pipeline-selector">
  <h2>Select Pipeline</h2>
  <div class="pipeline-options">
    <label class="pipeline-option selected">
      <input type="radio" name="pipeline" value="graphrag" checked>
      <div class="pipeline-info">
        <span class="pipeline-name">GraphRAG Pipeline</span>
        <span class="pipeline-description">Build knowledge graph from processed chunks...</span>
      </div>
    </label>
    <label class="pipeline-option">
      <input type="radio" name="pipeline" value="ingestion">
      <div class="pipeline-info">
        <span class="pipeline-name">Ingestion Pipeline</span>
        <span class="pipeline-description">Process raw video data through cleaning...</span>
      </div>
    </label>
  </div>
</div>
```

### 7.2 StageSelector

**Purpose:** Allow user to select which stages to run.

**Props:**
```typescript
interface StageSelectorProps {
  stages: Stage[];
  selectedStages: string[];
  onToggle: (stageName: string) => void;
}
```

**Behavior:**
- Display checkbox for each stage
- Show dependency information
- Warn when selecting stage without dependencies
- Option to auto-select dependencies

**Dependency Display:**
```html
<div class="stage-selector">
  <h2>Select Stages</h2>
  <div class="stage-list">
    <label class="stage-item">
      <input type="checkbox" name="stages" value="graph_extraction" checked>
      <span class="stage-name">Graph Extraction</span>
      <span class="stage-badge llm">LLM</span>
    </label>
    <label class="stage-item">
      <input type="checkbox" name="stages" value="entity_resolution" checked>
      <span class="stage-name">Entity Resolution</span>
      <span class="stage-deps">requires: Graph Extraction</span>
      <span class="stage-badge llm">LLM</span>
    </label>
    <!-- ... -->
  </div>
  
  <div class="dependency-warning" role="alert">
    <span class="warning-icon">⚠</span>
    <span>Entity Resolution requires Graph Extraction. It will be auto-included.</span>
  </div>
</div>
```

### 7.3 StageConfigPanel

**Purpose:** Render configuration form for a single stage.

**Props:**
```typescript
interface StageConfigPanelProps {
  stageName: string;
  schema: StageConfigSchema;
  config: Record<string, any>;
  defaults: Record<string, any>;
  expanded: boolean;
  onChange: (fieldName: string, value: any) => void;
  onToggleExpand: () => void;
  onReset: () => void;
}
```

**Structure:**
```html
<div class="stage-config-panel">
  <div class="panel-header" onclick="toggleExpand()">
    <span class="expand-icon">▼</span>
    <h3>Graph Extraction</h3>
    <span class="field-count">25 fields</span>
    <button class="reset-btn" title="Reset to defaults">↺</button>
  </div>
  
  <div class="panel-body">
    <!-- Category: Common Fields -->
    <div class="category-section">
      <h4>Common Fields</h4>
      <div class="field-grid">
        <!-- Fields rendered here -->
      </div>
    </div>
    
    <!-- Category: LLM Settings -->
    <div class="category-section">
      <h4>LLM Settings</h4>
      <div class="field-grid">
        <!-- Fields rendered here -->
      </div>
    </div>
    
    <!-- More categories... -->
  </div>
</div>
```

### 7.4 ConfigField

**Purpose:** Render a single configuration field based on its type.

**Props:**
```typescript
interface ConfigFieldProps {
  field: ConfigField;
  value: any;
  onChange: (value: any) => void;
}
```

**Field Type Mapping:**

| ui_type | HTML Element | Notes |
|---------|--------------|-------|
| `text` | `<input type="text">` | With placeholder |
| `number` | `<input type="number">` | With min/max/step |
| `slider` | `<input type="range">` | With value display |
| `checkbox` | `<input type="checkbox">` | Boolean toggle |
| `select` | `<select>` | Dropdown with options |
| `multiselect` | `<select multiple>` or checkboxes | Multiple selection |

**Field Template:**
```html
<div class="config-field">
  <label class="field-label">
    <span class="label-text">Model Name</span>
    <span class="field-type">string</span>
    <span class="recommended" title="Recommended value">★ gpt-4o-mini</span>
  </label>
  
  <!-- Input element varies by type -->
  <select class="field-input" name="model_name">
    <option value="gpt-4o-mini" selected>gpt-4o-mini</option>
    <option value="gpt-4o">gpt-4o</option>
    <option value="gpt-4">gpt-4</option>
  </select>
  
  <p class="field-description">OpenAI model to use for LLM operations</p>
</div>
```

### 7.5 ExecutionPanel

**Purpose:** Validate configuration, execute pipeline, show results.

**Props:**
```typescript
interface ExecutionPanelProps {
  canValidate: boolean;
  canExecute: boolean;
  validationResult: ValidationResult | null;
  executionResult: ExecutionResult | null;
  pipelineStatus: PipelineStatus | null;
  onValidate: () => void;
  onExecute: () => void;
  onCancel: () => void;
}
```

**Structure:**
```html
<div class="execution-panel">
  <div class="action-buttons">
    <button class="btn btn-secondary" onclick="validate()" disabled={!canValidate}>
      Validate Configuration
    </button>
    <button class="btn btn-primary" onclick="execute()" disabled={!canExecute}>
      Execute Pipeline
    </button>
  </div>
  
  <!-- Validation Results -->
  <div class="validation-results" role="status">
    <div class="result-header success">
      <span class="icon">✓</span>
      <span>Configuration Valid</span>
    </div>
    <div class="result-warnings">
      <div class="warning-item">
        <span class="icon">⚠</span>
        <span>Stage 'entity_resolution' depends on 'graph_extraction' (auto-included)</span>
      </div>
    </div>
    <div class="execution-plan">
      <strong>Execution Plan:</strong>
      graph_extraction → entity_resolution → graph_construction
    </div>
  </div>
  
  <!-- Execution Status (shown after execute) -->
  <div class="execution-status">
    <div class="status-header running">
      <span class="icon spinner">⟳</span>
      <span>Pipeline Running</span>
      <button class="cancel-btn" onclick="cancel()">Cancel</button>
    </div>
    <div class="status-details">
      <div>Pipeline ID: <code>pipeline_1234567890_abc123</code></div>
      <div>Current Stage: entity_resolution (2/3)</div>
      <div>Progress: 66%</div>
      <div class="progress-bar">
        <div class="progress-fill" style="width: 66%"></div>
      </div>
      <div>Elapsed: 45 seconds</div>
    </div>
  </div>
</div>
```

---

## 8. Form Generation System

### 8.1 Form Generator Logic

```javascript
// formGenerator.js

export function generateForm(schema, values, defaults) {
  const form = document.createElement('div');
  form.className = 'config-form';
  
  // Group fields by category
  for (const category of schema.categories) {
    const section = createCategorySection(category, schema.fields, values, defaults);
    form.appendChild(section);
  }
  
  return form;
}

function createCategorySection(category, allFields, values, defaults) {
  const section = document.createElement('div');
  section.className = 'category-section';
  
  // Category header
  const header = document.createElement('h4');
  header.textContent = category.name;
  section.appendChild(header);
  
  // Field grid
  const grid = document.createElement('div');
  grid.className = 'field-grid';
  
  // Get fields for this category
  const categoryFields = allFields.filter(f => category.fields.includes(f.name));
  
  for (const field of categoryFields) {
    const fieldElement = createField(field, values[field.name], defaults[field.name]);
    grid.appendChild(fieldElement);
  }
  
  section.appendChild(grid);
  return section;
}

function createField(field, value, defaultValue) {
  const container = document.createElement('div');
  container.className = 'config-field';
  container.dataset.field = field.name;
  
  // Label
  const label = createLabel(field);
  container.appendChild(label);
  
  // Input based on ui_type
  const input = createInput(field, value ?? defaultValue);
  container.appendChild(input);
  
  // Description
  if (field.description) {
    const desc = document.createElement('p');
    desc.className = 'field-description';
    desc.textContent = field.description;
    container.appendChild(desc);
  }
  
  return container;
}
```

### 8.2 Input Factory

```javascript
function createInput(field, value) {
  switch (field.ui_type) {
    case 'text':
      return createTextInput(field, value);
    case 'number':
      return createNumberInput(field, value);
    case 'slider':
      return createSliderInput(field, value);
    case 'checkbox':
      return createCheckboxInput(field, value);
    case 'select':
      return createSelectInput(field, value);
    case 'multiselect':
      return createMultiselectInput(field, value);
    default:
      return createTextInput(field, value);
  }
}

function createTextInput(field, value) {
  const input = document.createElement('input');
  input.type = 'text';
  input.name = field.name;
  input.value = value ?? '';
  input.placeholder = field.placeholder || '';
  input.className = 'field-input field-text';
  return input;
}

function createNumberInput(field, value) {
  const input = document.createElement('input');
  input.type = 'number';
  input.name = field.name;
  input.value = value ?? '';
  if (field.min !== undefined) input.min = field.min;
  if (field.max !== undefined) input.max = field.max;
  if (field.step !== undefined) input.step = field.step;
  input.placeholder = field.placeholder || '';
  input.className = 'field-input field-number';
  return input;
}

function createSliderInput(field, value) {
  const container = document.createElement('div');
  container.className = 'slider-container';
  
  const input = document.createElement('input');
  input.type = 'range';
  input.name = field.name;
  input.value = value ?? field.default ?? field.min ?? 0;
  input.min = field.min ?? 0;
  input.max = field.max ?? 1;
  input.step = field.step ?? 0.1;
  input.className = 'field-input field-slider';
  
  const display = document.createElement('span');
  display.className = 'slider-value';
  display.textContent = input.value;
  
  input.addEventListener('input', () => {
    display.textContent = input.value;
  });
  
  container.appendChild(input);
  container.appendChild(display);
  return container;
}

function createCheckboxInput(field, value) {
  const container = document.createElement('label');
  container.className = 'checkbox-container';
  
  const input = document.createElement('input');
  input.type = 'checkbox';
  input.name = field.name;
  input.checked = value ?? false;
  input.className = 'field-input field-checkbox';
  
  const checkmark = document.createElement('span');
  checkmark.className = 'checkmark';
  
  container.appendChild(input);
  container.appendChild(checkmark);
  return container;
}

function createSelectInput(field, value) {
  const select = document.createElement('select');
  select.name = field.name;
  select.className = 'field-input field-select';
  
  // Add empty option if field is optional
  if (field.optional) {
    const empty = document.createElement('option');
    empty.value = '';
    empty.textContent = '-- Select --';
    select.appendChild(empty);
  }
  
  for (const option of (field.options || [])) {
    const opt = document.createElement('option');
    opt.value = option;
    opt.textContent = option;
    opt.selected = option === value;
    
    // Mark recommended option
    if (option === field.recommended) {
      opt.textContent += ' ★';
    }
    
    select.appendChild(opt);
  }
  
  return select;
}

function createMultiselectInput(field, value) {
  const container = document.createElement('div');
  container.className = 'multiselect-container';
  
  const selectedValues = Array.isArray(value) ? value : [];
  
  for (const option of (field.options || [])) {
    const label = document.createElement('label');
    label.className = 'multiselect-option';
    
    const input = document.createElement('input');
    input.type = 'checkbox';
    input.name = field.name;
    input.value = option;
    input.checked = selectedValues.includes(option);
    
    label.appendChild(input);
    label.appendChild(document.createTextNode(option));
    container.appendChild(label);
  }
  
  return container;
}
```

---

## 9. Validation & Error Handling

### 9.1 Client-Side Validation

```javascript
// validation.js

export function validateField(field, value) {
  const errors = [];
  
  // Required check
  if (field.required && (value === null || value === undefined || value === '')) {
    errors.push(`${field.name} is required`);
    return errors;
  }
  
  // Skip further validation if empty and optional
  if (value === null || value === undefined || value === '') {
    return errors;
  }
  
  // Type validation
  switch (field.type) {
    case 'integer':
      if (!Number.isInteger(Number(value))) {
        errors.push(`${field.name} must be an integer`);
      }
      break;
    case 'number':
      if (isNaN(Number(value))) {
        errors.push(`${field.name} must be a number`);
      }
      break;
    case 'boolean':
      if (typeof value !== 'boolean') {
        errors.push(`${field.name} must be true or false`);
      }
      break;
  }
  
  // Range validation
  if (field.min !== undefined && Number(value) < field.min) {
    errors.push(`${field.name} must be at least ${field.min}`);
  }
  if (field.max !== undefined && Number(value) > field.max) {
    errors.push(`${field.name} must be at most ${field.max}`);
  }
  
  // Options validation
  if (field.options && !field.options.includes(value)) {
    errors.push(`${field.name} must be one of: ${field.options.join(', ')}`);
  }
  
  return errors;
}

export function validateForm(schema, values) {
  const allErrors = {};
  
  for (const field of schema.fields) {
    const errors = validateField(field, values[field.name]);
    if (errors.length > 0) {
      allErrors[field.name] = errors;
    }
  }
  
  return {
    valid: Object.keys(allErrors).length === 0,
    errors: allErrors
  };
}
```

### 9.2 Error Display

```html
<div class="field-error" role="alert">
  <span class="error-icon">✕</span>
  <span class="error-message">Temperature must be between 0 and 2</span>
</div>
```

```css
.config-field.has-error .field-input {
  border-color: var(--color-error);
}

.field-error {
  color: var(--color-error);
  font-size: 0.875rem;
  margin-top: 0.25rem;
  display: flex;
  align-items: center;
  gap: 0.25rem;
}
```

---

## 10. Execution & Status Monitoring

### 10.1 Execution Flow

```javascript
async function executePipeline() {
  // 1. Validate first
  const validation = await api.validatePipeline(
    state.selectedPipeline,
    state.selectedStages,
    state.stageConfigs
  );
  
  if (!validation.valid) {
    showValidationErrors(validation.errors);
    return;
  }
  
  // 2. Show warnings but continue
  if (validation.warnings.length > 0) {
    showWarnings(validation.warnings);
  }
  
  // 3. Execute pipeline
  const result = await api.executePipeline(
    state.selectedPipeline,
    validation.execution_plan.stages, // Use resolved stages
    state.stageConfigs,
    { experiment_id: generateExperimentId() }
  );
  
  if (result.error) {
    showError(result.error);
    return;
  }
  
  // 4. Start polling status
  state.currentPipelineId = result.pipeline_id;
  startStatusPolling(result.pipeline_id);
}
```

### 10.2 Status Polling

```javascript
let pollingInterval = null;

function startStatusPolling(pipelineId) {
  // Clear any existing polling
  if (pollingInterval) {
    clearInterval(pollingInterval);
  }
  
  // Poll every 2 seconds
  pollingInterval = setInterval(async () => {
    const status = await api.getPipelineStatus(pipelineId);
    
    updateStatusDisplay(status);
    
    // Stop polling if pipeline finished
    if (['completed', 'failed', 'error', 'cancelled'].includes(status.status)) {
      clearInterval(pollingInterval);
      pollingInterval = null;
      onPipelineComplete(status);
    }
  }, 2000);
}

function updateStatusDisplay(status) {
  const statusEl = document.querySelector('.execution-status');
  
  // Update status header
  statusEl.querySelector('.status-header').className = `status-header ${status.status}`;
  statusEl.querySelector('.status-text').textContent = getStatusText(status.status);
  
  // Update progress
  const progress = status.progress;
  statusEl.querySelector('.current-stage').textContent = status.current_stage || 'N/A';
  statusEl.querySelector('.progress-text').textContent = 
    `${progress.completed_stages}/${progress.total_stages} (${progress.percent.toFixed(0)}%)`;
  statusEl.querySelector('.progress-fill').style.width = `${progress.percent}%`;
  statusEl.querySelector('.elapsed').textContent = `${status.elapsed_seconds}s`;
  
  // Show/hide cancel button
  statusEl.querySelector('.cancel-btn').style.display = 
    ['running', 'starting'].includes(status.status) ? 'block' : 'none';
}

function getStatusText(status) {
  const statusMap = {
    starting: 'Starting...',
    running: 'Running',
    completed: 'Completed Successfully',
    failed: 'Failed',
    error: 'Error',
    cancelled: 'Cancelled'
  };
  return statusMap[status] || status;
}
```

---

## 11. Styling & UX Guidelines

### 11.1 Color Palette

```css
:root {
  /* Primary colors */
  --color-primary: #3B82F6;        /* Blue - primary actions */
  --color-primary-hover: #2563EB;
  --color-primary-light: #DBEAFE;
  
  /* Status colors */
  --color-success: #10B981;        /* Green - success */
  --color-warning: #F59E0B;        /* Amber - warnings */
  --color-error: #EF4444;          /* Red - errors */
  --color-info: #6366F1;           /* Indigo - info */
  
  /* Neutral colors */
  --color-bg: #F9FAFB;             /* Light gray background */
  --color-surface: #FFFFFF;         /* White surface */
  --color-border: #E5E7EB;         /* Gray border */
  --color-text: #1F2937;           /* Dark text */
  --color-text-secondary: #6B7280; /* Gray text */
  --color-text-muted: #9CA3AF;     /* Light gray text */
  
  /* Shadows */
  --shadow-sm: 0 1px 2px 0 rgb(0 0 0 / 0.05);
  --shadow: 0 1px 3px 0 rgb(0 0 0 / 0.1);
  --shadow-md: 0 4px 6px -1px rgb(0 0 0 / 0.1);
  
  /* Spacing */
  --space-1: 0.25rem;
  --space-2: 0.5rem;
  --space-3: 0.75rem;
  --space-4: 1rem;
  --space-6: 1.5rem;
  --space-8: 2rem;
  
  /* Border radius */
  --radius-sm: 0.25rem;
  --radius: 0.375rem;
  --radius-md: 0.5rem;
  --radius-lg: 0.75rem;
}
```

### 11.2 Typography

```css
body {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 
               'Helvetica Neue', Arial, sans-serif;
  font-size: 14px;
  line-height: 1.5;
  color: var(--color-text);
}

h1 { font-size: 1.5rem; font-weight: 600; }
h2 { font-size: 1.25rem; font-weight: 600; }
h3 { font-size: 1rem; font-weight: 600; }
h4 { font-size: 0.875rem; font-weight: 600; color: var(--color-text-secondary); }

.field-label { font-weight: 500; }
.field-description { font-size: 0.75rem; color: var(--color-text-muted); }
```

### 11.3 Component Styles

```css
/* Buttons */
.btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: var(--space-2) var(--space-4);
  font-size: 0.875rem;
  font-weight: 500;
  border-radius: var(--radius);
  cursor: pointer;
  transition: all 0.15s ease;
}

.btn-primary {
  background: var(--color-primary);
  color: white;
  border: none;
}

.btn-primary:hover:not(:disabled) {
  background: var(--color-primary-hover);
}

.btn-secondary {
  background: white;
  color: var(--color-text);
  border: 1px solid var(--color-border);
}

.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

/* Inputs */
.field-input {
  width: 100%;
  padding: var(--space-2) var(--space-3);
  border: 1px solid var(--color-border);
  border-radius: var(--radius);
  font-size: 0.875rem;
  transition: border-color 0.15s ease, box-shadow 0.15s ease;
}

.field-input:focus {
  outline: none;
  border-color: var(--color-primary);
  box-shadow: 0 0 0 3px var(--color-primary-light);
}

/* Panels */
.panel {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  padding: var(--space-4);
  margin-bottom: var(--space-4);
}

.panel-header {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  cursor: pointer;
  user-select: none;
}

/* Status indicators */
.status-indicator {
  display: inline-flex;
  align-items: center;
  gap: var(--space-1);
  padding: var(--space-1) var(--space-2);
  border-radius: var(--radius-sm);
  font-size: 0.75rem;
  font-weight: 500;
}

.status-indicator.running {
  background: var(--color-primary-light);
  color: var(--color-primary);
}

.status-indicator.completed {
  background: #D1FAE5;
  color: var(--color-success);
}

.status-indicator.failed {
  background: #FEE2E2;
  color: var(--color-error);
}
```

### 11.4 UX Best Practices

1. **Loading States**: Show skeleton loaders while fetching data
2. **Feedback**: Provide immediate visual feedback on interactions
3. **Defaults**: Pre-fill forms with sensible defaults from API
4. **Errors**: Display errors inline near the problematic field
5. **Confirmations**: Confirm destructive actions (cancel pipeline)
6. **Progress**: Show progress indicators for long operations
7. **Accessibility**: Use proper ARIA labels and keyboard navigation

---

## 12. Implementation Phases

### Phase 1: Core Structure (Day 1-2)
- [ ] Set up project structure
- [ ] Create HTML shell with layout
- [ ] Implement API client module
- [ ] Create basic CSS framework

### Phase 2: Pipeline & Stage Selection (Day 2-3)
- [ ] Implement PipelineSelector component
- [ ] Implement StageSelector component
- [ ] Add dependency warnings
- [ ] Wire up state management

### Phase 3: Configuration Forms (Day 3-5)
- [ ] Implement form generator
- [ ] Create all input types (text, number, slider, etc.)
- [ ] Add category grouping
- [ ] Implement expand/collapse panels
- [ ] Add reset to defaults functionality

### Phase 4: Validation (Day 5-6)
- [ ] Implement client-side validation
- [ ] Add server-side validation integration
- [ ] Display validation errors/warnings
- [ ] Show execution plan preview

### Phase 5: Execution & Monitoring (Day 6-7)
- [ ] Implement execute pipeline
- [ ] Add status polling
- [ ] Create progress display
- [ ] Add cancel functionality

### Phase 6: Polish (Day 7-8)
- [ ] Responsive design
- [ ] Loading states
- [ ] Error boundaries
- [ ] Accessibility audit
- [ ] Final styling

---

## Quick Reference

### API Endpoints Used by UI

| Endpoint | When Called | Purpose |
|----------|-------------|---------|
| `GET /stages` | Page load | Get all pipelines and stages |
| `GET /stages/{name}/config` | Stage selected | Get config schema for form |
| `GET /stages/{name}/defaults` | Stage selected | Get default values |
| `POST /pipelines/validate` | Validate clicked | Validate configuration |
| `POST /pipelines/execute` | Execute clicked | Start pipeline |
| `GET /pipelines/{id}/status` | Polling | Get execution status |
| `POST /pipelines/{id}/cancel` | Cancel clicked | Cancel running pipeline |

---

**End of UI Design Specification**

