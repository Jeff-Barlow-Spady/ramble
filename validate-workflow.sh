#!/bin/bash
set -e

WORKFLOW_FILE=${1:-".github/workflows/build.yml"}

echo "Validating GitHub Actions workflow file: $WORKFLOW_FILE"

# Check if yq is installed
if ! command -v yq &> /dev/null; then
    echo "yq not found, installing..."
    sudo apt-get update
    sudo apt-get install -y wget
    wget https://github.com/mikefarah/yq/releases/download/v4.34.2/yq_linux_amd64 -O /tmp/yq
    chmod +x /tmp/yq
    sudo mv /tmp/yq /usr/local/bin/yq
fi

# Verify the file exists
if [ ! -f "$WORKFLOW_FILE" ]; then
    echo "❌ Error: Workflow file not found: $WORKFLOW_FILE"
    exit 1
fi

# Check YAML syntax
if ! yq -e . "$WORKFLOW_FILE" > /dev/null 2>&1; then
    echo "❌ Error: Invalid YAML syntax in workflow file"
    exit 1
fi
echo "✅ YAML syntax is valid"

# Check for required top-level sections
if ! yq -e '.on' "$WORKFLOW_FILE" > /dev/null 2>&1; then
    echo "❌ Error: Missing 'on' trigger in workflow file"
    exit 1
fi
echo "✅ 'on' trigger is defined"

if ! yq -e '.jobs' "$WORKFLOW_FILE" > /dev/null 2>&1; then
    echo "❌ Error: Missing 'jobs' section in workflow file"
    exit 1
fi
echo "✅ 'jobs' section is defined"

# Check for common triggers
TRIGGERS=$(yq -e '.on | keys | .[]' "$WORKFLOW_FILE" 2>/dev/null | tr '\n' ' ' || echo "none")
echo "📌 Triggers defined: $TRIGGERS"

# Check for env variables
ENV_VARS=$(yq -e '.env | keys | .[]' "$WORKFLOW_FILE" 2>/dev/null | tr '\n' ' ' || echo "none")
echo "📌 Environment variables: $ENV_VARS"

# Count jobs
JOB_COUNT=$(yq -e '.jobs | keys | length' "$WORKFLOW_FILE" 2>/dev/null || echo "0")
echo "📌 Number of jobs: $JOB_COUNT"

# List job names
JOB_NAMES=$(yq -e '.jobs | keys | .[]' "$WORKFLOW_FILE" 2>/dev/null | tr '\n' ' ' || echo "none")
echo "📌 Job names: $JOB_NAMES"

# Check for submodule checkout
if grep -q "submodules:" "$WORKFLOW_FILE"; then
    echo "✅ Submodule checkout appears to be configured"
else
    echo "⚠️ Warning: No submodule checkout configuration found"
fi

# Check for specific common errors - JavaScript-style comments
# Avoiding false positives with URLs by using a more specific pattern
if grep -E "^[^:\"']*\/\/" "$WORKFLOW_FILE"; then
    echo "⚠️ Warning: Found JavaScript-style comments (//) which are not valid in YAML"
else
    echo "✅ No JavaScript-style comments detected"
fi

# Verify steps in each job
for job in $(yq -e '.jobs | keys | .[]' "$WORKFLOW_FILE" 2>/dev/null); do
    echo "📋 Checking job: $job"

    # Check if runs-on is defined
    if ! yq -e ".jobs.$job.runs-on" "$WORKFLOW_FILE" > /dev/null 2>&1; then
        echo "  ❌ Error: Missing 'runs-on' in job '$job'"
    else
        RUNS_ON=$(yq -e ".jobs.$job.runs-on" "$WORKFLOW_FILE")
        echo "  ✅ runs-on: $RUNS_ON"
    fi

    # Check if steps are defined
    if ! yq -e ".jobs.$job.steps" "$WORKFLOW_FILE" > /dev/null 2>&1; then
        echo "  ❌ Error: Missing 'steps' in job '$job'"
    else
        STEP_COUNT=$(yq -e ".jobs.$job.steps | length" "$WORKFLOW_FILE")
        echo "  ✅ steps: $STEP_COUNT step(s) defined"

        # Check checkout step
        if yq -e ".jobs.$job.steps[] | select(.uses == \"actions/checkout@v4\")" "$WORKFLOW_FILE" > /dev/null 2>&1; then
            echo "  ✅ Contains checkout step with actions/checkout@v4"
        else
            echo "  ⚠️ Warning: No checkout step with actions/checkout@v4 found"
        fi
    fi
done

echo "✅ Workflow validation complete"