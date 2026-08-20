# TODO Analysis

## Overview

The TODO analysis system provides automated tracking and categorization of remaining TODO comments in generated Terraform provider resource files. It generates a structured JSON report that helps developers understand what work remains and prioritize implementation tasks.

## Quick Start

Generate a TODO analysis report:

```bash
make analyze-todos
```

This creates `todo-analysis.json` in the project root with a complete breakdown of all TODOs.

## How It Works

### 1. Script Location

- **Script**: `scripts/analyze-todos.js`
- **Make target**: `analyze-todos`
- **Output**: `todo-analysis.json`

### 2. Scanning Process

The script:

1. Scans all resource files in `internal/provider/*_resource.go`
2. Excludes test files (`*_test.go`)
3. Extracts TODO comments and their context
4. Categorizes each TODO based on content patterns
5. Generates summary statistics
6. Outputs structured JSON report

### 3. TODO Categories

The script automatically categorizes TODOs into six types:

| Category | Count | Description | Example |
|----------|-------|-------------|---------|
| **list_conversion** | 84 | Array/list field conversions for Create/Update operations | `TODO: Convert AccountIds list to int64 array` |
| **update_placeholder** | 34 | Update response handling placeholders | `TODO: Update plan from API response if needed` |
| **unmarshal_list** | 17 | List unmarshaling from API responses in Read operations | `TODO: Unmarshal list aws_iam_policies to state` |
| **nested_object** | 11 | Nested object type conversions | `TODO: Handle nested object: azure_policy` |
| **handle_list_field** | 10 | Complex list field handling | `TODO: Handle list field: aws_session_tags` |
| **other** | 1 | Uncategorized TODOs | Any TODO not matching above patterns |

## Output Format

### JSON Structure

The output file `todo-analysis.json` has three main sections:

```json
{
  "generated_at": "2025-10-21T21:46:38.176Z",
  "summary": { /* ... */ },
  "files": [ /* ... */ ]
}
```

### Summary Section

Overall statistics across all resource files:

```json
{
  "summary": {
    "total_files_with_todos": 34,
    "total_files_scanned": 34,
    "total_todos": 157,
    "category_breakdown": {
      "list_conversion": 84,
      "update_placeholder": 34,
      "unmarshal_list": 17,
      "nested_object": 11,
      "handle_list_field": 10,
      "other": 1
    },
    "files_by_category": {
      "list_conversion": [
        {
          "resource_name": "project_cloud_access_role",
          "count": 8
        }
      ]
    }
  }
}
```

### Files Section

Per-file details including all TODOs with line numbers:

```json
{
  "files": [
    {
      "file_path": "internal/provider/project_cloud_access_role_resource.go",
      "resource_name": "project_cloud_access_role",
      "todo_count": 16,
      "category_breakdown": {
        "list_conversion": 8,
        "update_placeholder": 1,
        "handle_list_field": 2,
        "unmarshal_list": 5,
        "nested_object": 0,
        "other": 0
      },
      "todos": [
        {
          "line_number": 66,
          "text": "Convert AccountIds list to int64 array for \"account_ids\"",
          "category": "list_conversion",
          "full_line": "// TODO: Convert AccountIds list to int64 array for \"account_ids\""
        }
      ]
    }
  ]
}
```

## Usage Examples

### Basic Queries

**View summary statistics:**

```bash
jq '.summary' todo-analysis.json
```

**List all TODO categories and counts:**

```bash
jq '.summary.category_breakdown' todo-analysis.json
```

**Count total TODOs:**

```bash
jq '.summary.total_todos' todo-analysis.json
```

### Finding Resources

**Resources with most TODOs (top 5):**

```bash
jq '.files | sort_by(.todo_count) | reverse | .[0:5] | .[] | {resource_name, todo_count}' todo-analysis.json
```

**All resources with list_conversion TODOs:**

```bash
jq '.summary.files_by_category.list_conversion' todo-analysis.json
```

**Resources with no TODOs:**

```bash
jq '[.files[] | select(.todo_count == 0) | .resource_name]' todo-analysis.json
```

### Specific Resource Analysis

**Get all TODOs for a specific resource:**

```bash
jq '.files[] | select(.resource_name == "cloud_rule")' todo-analysis.json
```

**List TODOs with line numbers for a resource:**

```bash
jq '.files[] | select(.resource_name == "cloud_rule") | .todos[] | "\(.line_number): \(.text)"' todo-analysis.json
```

**Category breakdown for a resource:**

```bash
jq '.files[] | select(.resource_name == "cloud_rule") | .category_breakdown' todo-analysis.json
```

### Category-Based Queries

**All list_conversion TODOs across all files:**

```bash
jq '[.files[].todos[] | select(.category == "list_conversion")]' todo-analysis.json
```

**Count TODOs by category:**

```bash
jq '[.files[].todos[] | .category] | group_by(.) | map({category: .[0], count: length})' todo-analysis.json
```

**Find all nested_object TODOs with their locations:**

```bash
jq '.files[] | select(.category_breakdown.nested_object > 0) | {resource: .resource_name, todos: [.todos[] | select(.category == "nested_object") | {line: .line_number, text: .text}]}' todo-analysis.json
```

## Integration with Development Workflow

### 1. Before Starting Work

Generate a fresh report to see current TODO state:

```bash
make analyze-todos
```

### 2. Identify Work Items

Find resources that need attention:

```bash
# Find resources with many list conversions
jq '.files[] | select(.category_breakdown.list_conversion > 5) | {resource: .resource_name, count: .category_breakdown.list_conversion}' todo-analysis.json

# Find all nested object TODOs (may need helper functions)
jq '.files[] | select(.category_breakdown.nested_object > 0) | .resource_name' todo-analysis.json
```

### 3. Track Progress

After implementing TODOs, regenerate the report:

```bash
make analyze-todos
```

Compare before/after TODO counts:

```bash
# Save current count
BEFORE=$(jq '.summary.total_todos' todo-analysis.json)

# Make changes, then regenerate
make analyze-todos

# Check new count
AFTER=$(jq '.summary.total_todos' todo-analysis.json)

echo "Reduced TODOs from $BEFORE to $AFTER"
```

### 4. CI/CD Integration

Track TODO trends over time by committing the analysis:

```bash
# Generate report
make analyze-todos

# Commit with PR
git add todo-analysis.json
git commit -m "Update TODO analysis"
```

## Understanding TODO Categories in Detail

### List Conversion TODOs (84 total)

**Purpose**: Convert Terraform list types to Go arrays for API requests

**Example TODO:**

```go
// TODO: Convert AccountIds list to int64 array for "account_ids"
// Example: var account_idsList []int64
// plan.AccountIds.ElementsAs(ctx, &account_idsList, false)
```

**Implementation Pattern:**

```go
var accountIdsList []int64
diags := plan.AccountIds.ElementsAs(ctx, &accountIdsList, false)
resp.Diagnostics.Append(diags...)
if resp.Diagnostics.HasError() {
    return
}
createReq["account_ids"] = accountIdsList
```

### Update Placeholder TODOs (34 total)

**Purpose**: Handle computed fields returned from API after Update operations

**Example TODO:**

```go
// TODO: Update plan from API response if needed
```

**When Needed:**

- API returns computed fields (e.g., `last_updated_at`)
- API modifies submitted values (e.g., normalizes strings)
- API adds server-generated fields

**Implementation Pattern:**

```go
// After successful update, update computed fields
if updatedAt, ok := apiResp["updated_at"].(string); ok {
    plan.UpdatedAt = types.StringValue(updatedAt)
}
```

### Unmarshal List TODOs (17 total)

**Purpose**: Convert API response arrays back to Terraform list types

**Example TODO:**

```go
// TODO: Unmarshal list aws_iam_policies to state.AwsIamPolicies
```

**Implementation Pattern:**

```go
if policies, ok := apiResp["aws_iam_policies"].([]interface{}); ok {
    var policyIds []int64
    for _, p := range policies {
        if id, ok := p.(float64); ok {
            policyIds = append(policyIds, int64(id))
        }
    }
    state.AwsIamPolicies, _ = types.ListValueFrom(ctx, types.Int64Type, policyIds)
}
```

### Nested Object TODOs (11 total)

**Purpose**: Handle complex nested object types

**Example TODO:**

```go
// TODO: Handle nested object: azure_policy (type: AzurePolicyValue)
```

**Implementation**: Requires custom type conversion based on the nested object's structure.

### Handle List Field TODOs (10 total)

**Purpose**: Complex list fields requiring special handling (e.g., list of objects)

**Example TODO:**

```go
// TODO: Handle list field: aws_session_tags
```

**Implementation**: Depends on list item type (objects, maps, etc.)

## Generating Custom Reports

### Export TODOs to CSV

```bash
echo "Resource,Category,Line,Text" > todos.csv
jq -r '.files[] | .resource_name as $res | .todos[] | [$res, .category, .line_number, .text] | @csv' todo-analysis.json >> todos.csv
```

### Generate Markdown Checklist

```bash
echo "# TODO Checklist" > todos.md
jq -r '.files[] | "## \(.resource_name) (\(.todo_count) TODOs)\n" + (.todos[] | "- [ ] Line \(.line_number): \(.text)\n")' todo-analysis.json >> todos.md
```

### Priority Report (Resources with 10+ TODOs)

```bash
jq '.files[] | select(.todo_count >= 10) | {resource: .resource_name, todos: .todo_count, categories: .category_breakdown}' todo-analysis.json
```

## Continuous Improvement

### Adding New Categories

Edit `scripts/analyze-todos.js` to add new TODO patterns:

```javascript
const TODO_CATEGORIES = {
    LIST_CONVERSION: 'list_conversion',
    NESTED_OBJECT: 'nested_object',
    // Add new category
    YOUR_CATEGORY: 'your_category'
};

function categorizeTodo(todoText) {
    // Add detection logic
    if (todoText.includes('your pattern')) {
        return TODO_CATEGORIES.YOUR_CATEGORY;
    }
    // ... existing logic
}
```

### Tracking TODO Trends

Commit `todo-analysis.json` after major changes to track progress:

```bash
# View TODO count over time
git log --oneline -- todo-analysis.json | while read commit msg; do
    count=$(git show $commit:todo-analysis.json | jq '.summary.total_todos')
    echo "$commit: $count TODOs - $msg"
done
```

## Related Documentation

- [Field Mapping Generation](./CONFIG-GENERATION.md) - How field mappings are auto-generated
- [Testing Strategy](./TESTING.md) - Testing generated resources
- [Coverage Tracking](./coverage-tracking.md) - Resource implementation coverage

## Troubleshooting

### Report shows 0 TODOs but I see them in files

- Ensure you're looking at `*_resource.go` files (not test files)
- Check TODO format: Must be `// TODO:` with colon
- Re-run: `make analyze-todos`

### Category is "other" instead of expected category

- Check the categorization logic in `scripts/analyze-todos.js`
- TODO text may not match existing patterns
- Add custom pattern for your use case

### Script fails to run

```bash
# Ensure script is executable
chmod +x scripts/analyze-todos.js

# Check Node.js is installed
node --version

# Run directly
node scripts/analyze-todos.js
```

## Summary

The TODO analysis system provides:

✅ **Automatic categorization** of 157 remaining TODOs
✅ **Per-resource breakdown** showing what work remains
✅ **Structured JSON output** for programmatic querying
✅ **Progress tracking** over time
✅ **Integration with development workflow**

Use `make analyze-todos` whenever you need to understand the current state of remaining implementation work.
