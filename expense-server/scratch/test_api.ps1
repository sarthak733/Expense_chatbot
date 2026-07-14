param()
# test_api.ps1 — Run from expense-server\ directory
# Usage: .\scratch\test_api.ps1

$BASE = "http://localhost:8080"
$PASS = 0
$FAIL = 0

function Call-RPC($uri, $hdrs, $body) {
    try { return Invoke-RestMethod -Uri $uri -Method Post -Headers $hdrs -Body $body -EA Stop }
    catch { 
        if ($_.ErrorDetails.Message) {
            return $_.ErrorDetails.Message | ConvertFrom-Json -EA SilentlyContinue
        }
        return @{ "code" = "error"; "message" = $_.Exception.Message }
    }
}

function Show-Pass($name) { Write-Host "  [PASS] $name" -ForegroundColor Green;  $script:PASS++ }
function Show-Fail($name, $detail) { Write-Host "  [FAIL] $name => $detail" -ForegroundColor Red; $script:FAIL++ }

Write-Host "`nexpense-server Test Suite (Phase 3)" -ForegroundColor Cyan
Write-Host "==================================" -ForegroundColor Cyan

# ------------------------------------------------------------------
Write-Host "`n[1] Register & Login" -ForegroundColor Yellow
# ------------------------------------------------------------------
$rand = Get-Random -Maximum 100000
$username1 = "user1_$rand"
$username2 = "user2_$rand"
$userPass = "securepass123"

$noAuth = @{ "Content-Type" = "application/json" }

# Register User 1
$r = Call-RPC "$BASE/expense.v1.AuthService/Register" $noAuth "{`"username`":`"$username1`",`"password`":`"$userPass`"}"
if ($r.token) { Show-Pass "Register succeeds"; $token1 = $r.token }
else { Show-Fail "Register" ($r | ConvertTo-Json -Compress) }

# Register User 2
$r = Call-RPC "$BASE/expense.v1.AuthService/Register" $noAuth "{`"username`":`"$username2`",`"password`":`"$userPass`"}"
$token2 = $r.token

# Login User 1
$r = Call-RPC "$BASE/expense.v1.AuthService/Login" $noAuth "{`"username`":`"$username1`",`"password`":`"$userPass`"}"
if ($r.token) { Show-Pass "Login succeeds"; $token1 = $r.token }
else { Show-Fail "Login" ($r | ConvertTo-Json -Compress) }

$h1 = @{ "Authorization" = "Bearer $token1"; "Content-Type" = "application/json" }
$h2 = @{ "Authorization" = "Bearer $token2"; "Content-Type" = "application/json" }

# ------------------------------------------------------------------
Write-Host "`n[2] Categories (CRUD)" -ForegroundColor Yellow
# ------------------------------------------------------------------
# Create Category 1
$r = Call-RPC "$BASE/expense.v1.ExpenseService/CreateCategory" $h1 '{"name":"Travel","color":"#0000FF"}'
if ($r.category.id -gt 0) { 
    Show-Pass "Create Category succeeds"
    $catID = $r.category.id 
} else { 
    Show-Fail "Create Category" ($r | ConvertTo-Json -Compress) 
}

# List Categories (verify custom + system defaults exist)
$r = Call-RPC "$BASE/expense.v1.ExpenseService/ListCategories" $h1 '{}'
$hasCustom = $false
$hasFood = $false
foreach ($cat in $r.categories) {
    if ($cat.name -eq "Travel") { $hasCustom = $true }
    if ($cat.name -eq "Food") { $hasFood = $true }
}
if ($hasCustom -and $hasFood) { Show-Pass "List Categories retrieves system + custom categories" }
else { Show-Fail "List Categories" "Custom found: $hasCustom | Default Food found: $hasFood" }

# Update Category
$r = Call-RPC "$BASE/expense.v1.ExpenseService/UpdateCategory" $h1 "{`"id`":$catID,`"name`":`"Transportation`",`"color`":`"#00FF00`"}"
if ($r.category.name -eq "Transportation") { Show-Pass "Update Category succeeds" }
else { Show-Fail "Update Category" ($r | ConvertTo-Json -Compress) }

# ------------------------------------------------------------------
Write-Host "`n[3] Create Expense with Category" -ForegroundColor Yellow
# ------------------------------------------------------------------
# Create expense with resolved category string
$r = Call-RPC "$BASE/expense.v1.ExpenseService/CreateExpense" $h1 "{`"title`":`"Uber Ride`",`"amount`":15.5,`"category`":`"Transportation`"}"
if ($r.expense.categoryId -eq $catID -and $r.expense.category -eq "Transportation") {
    Show-Pass "CreateExpense resolves category by name successfully"
    $expID = $r.expense.id
} else {
    Show-Fail "CreateExpense Resolution" "Expected Category ID $catID, got $($r.expense.categoryId) | Response: $($r | ConvertTo-Json -Compress)"
}

# ------------------------------------------------------------------
Write-Host "`n[4] NLP Parsing & Quick Add" -ForegroundColor Yellow
# ------------------------------------------------------------------
# Test ParseExpenseText
$r = Call-RPC "$BASE/expense.v1.ExpenseService/ParseExpenseText" $h1 '{"text":"spent 25 dollars on Starbucks coffee yesterday"}'
if ($r.amount -eq 25 -and $r.title -eq "Starbucks coffee" -and $r.category -eq "Food") {
    Show-Pass "ParseExpenseText extracts amount, title, and maps category correctly"
} else {
    Show-Fail "ParseExpenseText" ($r | ConvertTo-Json -Compress)
}

# Test QuickAddExpense (actually inserts)
$r = Call-RPC "$BASE/expense.v1.ExpenseService/QuickAddExpense" $h1 '{"text":"spent 15 dollars on Uber ride yesterday"}'
if ($r.expense.id -gt 0 -and $r.expense.amount -eq 15 -and $r.expense.category -eq "Transport") {
    Show-Pass "QuickAddExpense parses and inserts expense with dynamic category mapping"
} else {
    Show-Fail "QuickAddExpense" ($r | ConvertTo-Json -Compress)
}

# ------------------------------------------------------------------
Write-Host "`n[5] Budgets with Status Tracking" -ForegroundColor Yellow
# ------------------------------------------------------------------
$start = (Get-Date).AddDays(-5).ToString("yyyy-MM-dd")
$end = (Get-Date).AddDays(25).ToString("yyyy-MM-dd")

# Create a monthly budget
$r = Call-RPC "$BASE/expense.v1.ExpenseService/CreateBudget" $h1 "{`"categoryId`":0,`"amount`":100,`"period`":`"monthly`",`"startDate`":`"$start`",`"endDate`":`"$end`"}"
if ($r.budget.id -gt 0) { 
    Show-Pass "CreateBudget succeeds"
    $budgetID = $r.budget.id
} else { 
    Show-Fail "CreateBudget" ($r | ConvertTo-Json -Compress) 
}

# Add expense that exceeds 80% (warning) or 100% (exceeded)
# We already have Uber Ride ($15.5) and QuickAdd Uber ($15.0) which is $30.5 spent.
# Let's add an expense of $55.0 to make it $85.5 spent (85.5% -> warning status)
$r = Call-RPC "$BASE/expense.v1.ExpenseService/CreateExpense" $h1 "{`"title`":`"Flight`",`"amount`":55.0,`"categoryId`":$catID}"

$r = Call-RPC "$BASE/expense.v1.ExpenseService/ListBudgets" $h1 '{}'
$budget = $null
foreach ($b in $r.budgets) {
    if ($b.id -eq $budgetID) { $budget = $b }
}
if ($budget -and $budget.status -eq "warning") {
    Show-Pass "Budget status tracking identifies warning threshold (>80%)"
} else {
    Show-Fail "Budget Status Warning" "Expected status 'warning' (spent: $($budget.spent)/$($budget.amount)), got '$($budget.status)'"
}

# Add another expense to exceed 100%
$r = Call-RPC "$BASE/expense.v1.ExpenseService/CreateExpense" $h1 "{`"title`":`"Train`",`"amount`":20.0,`"categoryId`":$catID}"

$r = Call-RPC "$BASE/expense.v1.ExpenseService/ListBudgets" $h1 '{}'
$budget = $null
foreach ($b in $r.budgets) {
    if ($b.id -eq $budgetID) { $budget = $b }
}
if ($budget -and $budget.status -eq "exceeded") {
    Show-Pass "Budget status tracking identifies exceeded threshold (>100%)"
} else {
    Show-Fail "Budget Status Exceeded" "Expected status 'exceeded' (spent: $($budget.spent)/$($budget.amount)), got '$($budget.status)'"
}

# ------------------------------------------------------------------
Write-Host "`n[6] Recurring Expenses & Background Scheduler" -ForegroundColor Yellow
# ------------------------------------------------------------------
# Create a recurring expense scheduled for yesterday
$yesterdayStr = (Get-Date).AddDays(-1).ToString("yyyy-MM-dd")
$r = Call-RPC "$BASE/expense.v1.ExpenseService/CreateRecurringExpense" $h1 "{`"title`":`"Monthly Netflix`",`"amount`":15.0,`"categoryId`":$catID,`"interval`":`"daily`",`"nextRunDate`":`"$yesterdayStr`"}"
if ($r.recurringExpense.id -gt 0) { 
    Show-Pass "CreateRecurringExpense succeeds"
    $recurID = $r.recurringExpense.id
} else { 
    Show-Fail "CreateRecurringExpense" ($r | ConvertTo-Json -Compress) 
}

# Triggering Background Scheduler check:
# We configured the server to run scheduler immediate check on startup and every SCHEDULER_INTERVAL.
# Since we can't easily restart the server, wait! If SCHEDULER_INTERVAL was configured to 2s, 
# it will tick every 2 seconds in background automatically! Let's wait 3 seconds to let the ticker run.
Write-Host "  Waiting for background scheduler ticker to process recurring expense..." -ForegroundColor Gray
Start-Sleep -Seconds 3

# Check if the expense was auto-created
$r = Call-RPC "$BASE/expense.v1.ExpenseService/ListExpenses" $h1 '{}'
$foundAutoCreated = $false
foreach ($exp in $r.expenses) {
    if ($exp.title -eq "Monthly Netflix" -and $exp.amount -eq 15.0) {
        $foundAutoCreated = $true
    }
}
if ($foundAutoCreated) {
    Show-Pass "Background scheduler auto-created expense for due recurring items successfully"
} else {
    Show-Fail "Scheduler Execution" "Could not find auto-created expense with title 'Monthly Netflix' in user expenses list"
}

# Check if recurring expense next run date was advanced
$r = Call-RPC "$BASE/expense.v1.ExpenseService/ListRecurringExpenses" $h1 '{}'
$recur = $null
foreach ($rc in $r.recurringExpenses) {
    if ($rc.id -eq $recurID) { $recur = $rc }
}
$todayStr = (Get-Date).ToString("yyyy-MM-dd")
if ($recur -and $recur.nextRunDate -eq $todayStr) {
    Show-Pass "Recurring expense next run date advanced to today ($todayStr)"
} else {
    Show-Fail "Recurring Date Advance" "Expected next run date to be advanced to today ($todayStr), got: $($recur.nextRunDate)"
}

# ------------------------------------------------------------------
Write-Host "`n[7] Export to CSV/PDF" -ForegroundColor Yellow
# ------------------------------------------------------------------
$r = Call-RPC "$BASE/expense.v1.ExpenseService/ExportExpensesCSV" $h1 '{}'
if ($r.csvData) { Show-Pass "ExportExpensesCSV returns non-empty byte stream" }
else { Show-Fail "Export CSV" ($r | ConvertTo-Json -Compress) }

$r = Call-RPC "$BASE/expense.v1.ExpenseService/ExportExpensesPDF" $h1 '{}'
if ($r.pdfData) { Show-Pass "ExportExpensesPDF returns non-empty PDF binary stream" }
else { Show-Fail "Export PDF" ($r | ConvertTo-Json -Compress) }

# ------------------------------------------------------------------
Write-Host "`n[8] Monthly Comparison Insights" -ForegroundColor Yellow
# ------------------------------------------------------------------
$r = Call-RPC "$BASE/expense.v1.ExpenseService/GetMonthlyInsights" $h1 '{}'
if ($r.currentTotal -gt 0 -and $r.generalInsights.Length -gt 0) {
    Show-Pass "GetMonthlyInsights returns spend details and textual insights"
} else {
    Show-Fail "GetMonthlyInsights" ($r | ConvertTo-Json -Compress)
}

# ------------------------------------------------------------------
Write-Host "`n[9] Clean Up & Session Invalidation" -ForegroundColor Yellow
# ------------------------------------------------------------------
# Delete category
$r = Call-RPC "$BASE/expense.v1.ExpenseService/DeleteCategory" $h1 "{`"id`":$catID}"
if ($r.message -eq "category deleted successfully") { Show-Pass "DeleteCategory succeeds" }
else { Show-Fail "DeleteCategory" ($r | ConvertTo-Json -Compress) }

# Logout User 1
$r = Call-RPC "$BASE/expense.v1.AuthService/Logout" $h1 '{}'
if ($r.message -eq "Logged out successfully") { Show-Pass "Logout succeeds" }
else { Show-Fail "Logout" ($r | ConvertTo-Json -Compress) }

$r = Call-RPC "$BASE/expense.v1.ExpenseService/ListExpenses" $h1 '{}'
if ($r.code -eq "unauthenticated") { Show-Pass "Token is actively blocked after logout" }
else { Show-Fail "Logout bypass" "Token still works after logout!" }

# ------------------------------------------------------------------
Write-Host "`n==========================" -ForegroundColor Cyan
$total = $PASS + $FAIL
if ($FAIL -eq 0) { Write-Host "ALL $total TESTS PASSED" -ForegroundColor Green }
else {
    Write-Host "PASSED: $PASS / $total" -ForegroundColor Yellow
    Write-Host "FAILED: $FAIL / $total" -ForegroundColor Red
}
Write-Host ""
