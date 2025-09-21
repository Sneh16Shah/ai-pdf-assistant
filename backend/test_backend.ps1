# AI PDF Assistant Backend Test Suite
# PowerShell script to test all backend endpoints

Write-Host "`n========================================" -ForegroundColor Cyan
Write-Host "   AI PDF Assistant Backend Tests" -ForegroundColor Cyan
Write-Host "========================================`n" -ForegroundColor Cyan

$baseUrl = "http://localhost:8080/api/v1"
$pdfPath = "C:\Users\snehs\Downloads\IJRTI2304061.pdf"

# Test results storage
$testResults = @{
    Passed = 0
    Failed = 0
    Results = @()
}

# Helper function to run tests
function Test-Endpoint {
    param(
        [string]$TestName,
        [string]$Method,
        [string]$Endpoint,
        [object]$Body = $null,
        [hashtable]$Headers = @{"Content-Type" = "application/json"}
    )
    
    Write-Host "`n🧪 Testing: $TestName" -ForegroundColor Yellow
    Write-Host "   Endpoint: $Method $Endpoint" -ForegroundColor Gray
    
    try {
        $params = @{
            Uri = "$baseUrl$Endpoint"
            Method = $Method
            Headers = $Headers
            ErrorAction = "Stop"
        }
        
        if ($Body) {
            if ($Method -eq "POST" -or $Method -eq "PUT") {
                $params.Body = $Body | ConvertTo-Json -Depth 10
            }
        }
        
        $response = Invoke-RestMethod @params
        
        Write-Host "   ✅ PASSED" -ForegroundColor Green
        Write-Host "   Response:" -ForegroundColor Gray
        $response | ConvertTo-Json -Depth 10 | Write-Host -ForegroundColor DarkGray
        
        $script:testResults.Passed++
        $script:testResults.Results += @{
            Test = $TestName
            Status = "PASSED"
            Response = $response
        }
        
        return $response
    }
    catch {
        Write-Host "   ❌ FAILED" -ForegroundColor Red
        Write-Host "   Error: $($_.Exception.Message)" -ForegroundColor Red
        
        $script:testResults.Failed++
        $script:testResults.Results += @{
            Test = $TestName
            Status = "FAILED"
            Error = $_.Exception.Message
        }
        
        return $null
    }
}

# Test 1: Health Check
Write-Host "`n📋 PHASE 1: Basic Health Check" -ForegroundColor Magenta
Write-Host "================================" -ForegroundColor Magenta

Test-Endpoint -TestName "Health Check" -Method "GET" -Endpoint "/health"

# Test 2: PDF Text Extraction
Write-Host "`n📋 PHASE 2: PDF Processing" -ForegroundColor Magenta
Write-Host "===========================" -ForegroundColor Magenta

# Check if PDF exists
if (Test-Path $pdfPath) {
    Write-Host "   📄 PDF File Found: $pdfPath" -ForegroundColor Green
    
    $extractBody = @{
        file_path = $pdfPath
    }
    
    $pdfResponse = Test-Endpoint -TestName "PDF Text Extraction" `
                                 -Method "POST" `
                                 -Endpoint "/pdf/extract-text" `
                                 -Body $extractBody
    
    if ($pdfResponse) {
        $documentId = $pdfResponse.document_id
        $sessionId = $pdfResponse.session_id
        
        Write-Host "`n   📊 PDF Processing Results:" -ForegroundColor Cyan
        Write-Host "      Document ID: $documentId" -ForegroundColor White
        Write-Host "      Session ID: $sessionId" -ForegroundColor White
        Write-Host "      Pages: $($pdfResponse.pages)" -ForegroundColor White
        Write-Host "      Chunks: $($pdfResponse.chunks)" -ForegroundColor White
        
        # Store for later tests
        $script:testDocumentId = $documentId
        $script:testSessionId = $sessionId
    }
} else {
    Write-Host "   ⚠️  PDF File Not Found: $pdfPath" -ForegroundColor Yellow
}

# Test 3: PDF Status Check
if ($script:testDocumentId) {
    Write-Host "`n📋 PHASE 3: Document Status Check" -ForegroundColor Magenta
    Write-Host "===================================" -ForegroundColor Magenta
    
    Test-Endpoint -TestName "PDF Status Check" `
                  -Method "GET" `
                  -Endpoint "/pdf/status/$($script:testDocumentId)"
}

# Test 4: Chat Functionality
if ($script:testSessionId) {
    Write-Host "`n📋 PHASE 4: Chat System Testing" -ForegroundColor Magenta
    Write-Host "================================" -ForegroundColor Magenta
    
    # Test sending a message
    $chatBody = @{
        session_id = $script:testSessionId
        message = "What is this research paper about? Give me a brief summary of the main topics covered."
    }
    
    $chatResponse = Test-Endpoint -TestName "Send Chat Message" `
                                  -Method "POST" `
                                  -Endpoint "/chat/message" `
                                  -Body $chatBody
    
    if ($chatResponse) {
        Write-Host "`n   💬 AI Response Preview:" -ForegroundColor Cyan
        $preview = $chatResponse.response
        if ($preview.Length -gt 200) {
            $preview = $preview.Substring(0, 200) + "..."
        }
        Write-Host "      $preview" -ForegroundColor White
    }
    
    # Test chat history retrieval
    Start-Sleep -Seconds 1
    Test-Endpoint -TestName "Get Chat History" `
                  -Method "GET" `
                  -Endpoint "/chat/history/$($script:testSessionId)"
}

# Test 5: Multiple Chat Messages
if ($script:testSessionId) {
    Write-Host "`n📋 PHASE 5: Conversation Flow Testing" -ForegroundColor Magenta
    Write-Host "======================================" -ForegroundColor Magenta
    
    $questions = @(
        "What are the main AI techniques discussed in the paper?",
        "What applications of AI are mentioned?",
        "Who are the authors of this paper?"
    )
    
    foreach ($question in $questions) {
        $chatBody = @{
            session_id = $script:testSessionId
            message = $question
        }
        
        Write-Host "`n   📝 Question: $question" -ForegroundColor Cyan
        $response = Test-Endpoint -TestName "Follow-up Question" `
                                 -Method "POST" `
                                 -Endpoint "/chat/message" `
                                 -Body $chatBody
        
        Start-Sleep -Seconds 1  # Rate limiting
    }
}

# Test 6: Session Management
if ($script:testSessionId) {
    Write-Host "`n📋 PHASE 6: Session Management" -ForegroundColor Magenta
    Write-Host "===============================" -ForegroundColor Magenta
    
    # Get final chat history
    $historyResponse = Test-Endpoint -TestName "Final Chat History" `
                                     -Method "GET" `
                                     -Endpoint "/chat/history/$($script:testSessionId)"
    
    if ($historyResponse) {
        Write-Host "`n   📈 Conversation Statistics:" -ForegroundColor Cyan
        Write-Host "      Total Messages: $($historyResponse.messages.Count)" -ForegroundColor White
        Write-Host "      PDF: $($historyResponse.pdf_info.filename)" -ForegroundColor White
        Write-Host "      Pages: $($historyResponse.pdf_info.pages)" -ForegroundColor White
    }
    
    # Clear session
    Test-Endpoint -TestName "Clear Session" `
                  -Method "DELETE" `
                  -Endpoint "/session/$($script:testSessionId)"
}

# Test 7: Error Handling
Write-Host "`n📋 PHASE 7: Error Handling Tests" -ForegroundColor Magenta
Write-Host "==================================" -ForegroundColor Magenta

# Test with invalid session ID
$invalidBody = @{
    session_id = "invalid_session_123"
    message = "Test message"
}

Test-Endpoint -TestName "Invalid Session ID" `
              -Method "POST" `
              -Endpoint "/chat/message" `
              -Body $invalidBody

# Test with non-existent file
$invalidFileBody = @{
    file_path = "C:\non\existent\file.pdf"
}

Test-Endpoint -TestName "Non-existent File" `
              -Method "POST" `
              -Endpoint "/pdf/extract-text" `
              -Body $invalidFileBody

# Test with missing parameters
Test-Endpoint -TestName "Missing Parameters" `
              -Method "POST" `
              -Endpoint "/chat/message" `
              -Body @{}

# Final Test Summary
Write-Host "`n`n========================================" -ForegroundColor Cyan
Write-Host "         TEST SUMMARY REPORT" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

Write-Host "`n📊 Test Results:" -ForegroundColor Yellow
Write-Host "   ✅ Passed: $($testResults.Passed)" -ForegroundColor Green
Write-Host "   ❌ Failed: $($testResults.Failed)" -ForegroundColor Red
Write-Host "   📝 Total:  $($testResults.Passed + $testResults.Failed)" -ForegroundColor White

$passRate = if (($testResults.Passed + $testResults.Failed) -gt 0) {
    [math]::Round(($testResults.Passed / ($testResults.Passed + $testResults.Failed)) * 100, 2)
} else { 0 }

Write-Host "`n   Success Rate: $passRate%" -ForegroundColor $(if ($passRate -ge 80) { "Green" } elseif ($passRate -ge 60) { "Yellow" } else { "Red" })

# Performance check
Write-Host "`n⚡ Performance Metrics:" -ForegroundColor Yellow
$processInfo = Get-Process -Id (Get-Process -Name "pdf-assistant" -ErrorAction SilentlyContinue).Id -ErrorAction SilentlyContinue
if ($processInfo) {
    Write-Host "   Memory Usage: $([math]::Round($processInfo.WorkingSet64 / 1MB, 2)) MB" -ForegroundColor White
    Write-Host "   CPU Time: $([math]::Round($processInfo.TotalProcessorTime.TotalSeconds, 2)) seconds" -ForegroundColor White
}

# API Summary
Write-Host "`n🔧 Tested Endpoints:" -ForegroundColor Yellow
$endpoints = @(
    "GET  /api/v1/health",
    "POST /api/v1/pdf/extract-text",
    "GET  /api/v1/pdf/status/:id",
    "POST /api/v1/chat/message",
    "GET  /api/v1/chat/history/:sessionId",
    "DELETE /api/v1/session/:sessionId"
)

foreach ($endpoint in $endpoints) {
    $endpointParts = $endpoint -split ' '
    if ($endpointParts.Length -ge 2) {
        $pathParts = $endpointParts[1] -split '/'
        $lastPart = $pathParts[-1]
        $tested = $testResults.Results | Where-Object { $_.Test -like "*$lastPart*" }
        if ($tested) {
            Write-Host "   ✓ $endpoint" -ForegroundColor Green
        } else {
            Write-Host "   ○ $endpoint" -ForegroundColor Gray
        }
    }
}

# Save test results to file
$timestamp = Get-Date -Format "yyyy-MM-dd_HH-mm-ss"
$resultFile = "test_results_$timestamp.json"
$testResults | ConvertTo-Json -Depth 10 | Out-File $resultFile

Write-Host "`n📁 Test results saved to: $resultFile" -ForegroundColor Cyan
Write-Host "`n✨ Testing Complete!`n" -ForegroundColor Green