# AI Prompts Documentation
## AskMyPDF - AI-Powered PDF & FAQ Chatbot Platform

**Version:** 1.0  
**Date:** 2024

---

## Overview

This document describes the AI prompts used in AskMyPDF for question answering and summary generation. These prompts are designed to ensure the AI responds accurately based only on the provided document content.

---

## 1. Question Answering Prompt

### Purpose
Answer user questions based exclusively on the provided document context.

### Prompt Structure

```
System Message:
"You are a helpful assistant that answers questions based ONLY on the provided document context. If the answer is not in the context, say 'I cannot find this information in the document.'"

User Message:
"Document Context:

[Chunk 1]
[Relevant text from document chunk 1]

[Chunk 2]
[Relevant text from document chunk 2]

[Chunk 3]
[Relevant text from document chunk 3]

Previous conversation:
- User: [Previous question]
- Assistant: [Previous answer]

Question: {user_question}

Answer the question based ONLY on the document context above. If the answer is not in the context, respond with: 'I cannot find this information in the document.'"
```

### Key Features
- **Context Limitation**: Explicitly instructs AI to use only provided context
- **Fallback Response**: Clear instruction for "not found" scenarios
- **Conversation History**: Includes recent conversation for context
- **Chunk Selection**: Uses top 3-5 most relevant chunks via vector search

### Implementation Location
- **File**: `backend/infrastructure/services/ai_service.go`
- **Function**: `buildQuestionPrompt()`

### Example Usage

**Input:**
```
Context: "The company was founded in 2020. It specializes in AI technology."
Question: "When was the company founded?"
```

**Expected Output:**
```
"The company was founded in 2020."
```

**Input (Not Found):**
```
Context: "The company specializes in AI technology."
Question: "What is the CEO's name?"
```

**Expected Output:**
```
"I cannot find this information in the document."
```

---

## 2. Summary Generation Prompt

### Purpose
Generate comprehensive summaries of PDF documents in structured format.

### Prompt Structure

```
System Message:
"You are a helpful assistant that creates concise summaries in bullet point format."

User Message:
"Please provide a comprehensive summary of the following document in bullet point format.

Include:
1. Main topics and themes
2. Key takeaways
3. Important details

Document:
{document_text}

Format your response as:
- Summary: [brief overview]
- Key Takeaways:
  • [takeaway 1]
  • [takeaway 2]
- Main Topics:
  • [topic 1]
  • [topic 2]"
```

### Key Features
- **Structured Format**: Requests specific bullet-point format
- **Comprehensive Coverage**: Asks for topics, takeaways, and details
- **Length Limitation**: Document text truncated to 8000 characters for prompt efficiency
- **Parseable Output**: Structured format for easy parsing

### Implementation Location
- **File**: `backend/infrastructure/services/ai_service.go`
- **Function**: `buildSummaryPrompt()`

### Example Usage

**Input:**
```
Document: "This research paper discusses machine learning applications in healthcare. Key findings include improved diagnosis accuracy by 30% and reduced treatment costs by 20%. The study involved 1000 patients across 5 hospitals."
```

**Expected Output:**
```
Summary: This research paper explores machine learning applications in healthcare, demonstrating significant improvements in diagnosis accuracy and cost reduction.

Key Takeaways:
• Machine learning improves diagnosis accuracy by 30%
• Treatment costs reduced by 20%
• Study involved 1000 patients across 5 hospitals

Main Topics:
• Machine Learning
• Healthcare Applications
• Medical Diagnosis
• Cost Reduction
```

---

## 3. Guardrails and Safety Prompts

### Purpose
Ensure AI responses stay within document boundaries and don't hallucinate.

### Guardrail Checks

1. **Answer Validation**
   - Check if response contains "cannot find" or "not in the document"
   - Flag responses that seem to reference external knowledge
   - Validate answer relevance to question

2. **Context Adherence**
   - Verify answer can be traced to provided chunks
   - Reject answers that introduce new information not in context

3. **Response Formatting**
   - Ensure responses are clear and concise
   - Maintain professional tone
   - Avoid speculation

### Implementation Location
- **File**: `backend/infrastructure/services/ai_service.go`
- **Function**: `AnswerQuestion()` - answer validation logic

---

## 4. Prompt Engineering Best Practices

### Context Assembly
- **Chunk Selection**: Use vector search to select top 3-5 most relevant chunks
- **Chunk Ordering**: Present chunks in relevance order
- **Context Length**: Limit total context to ~4000-8000 tokens for efficiency

### Conversation History
- **History Length**: Include last 4-6 messages for context
- **Format**: Clear user/assistant distinction
- **Relevance**: Only include relevant previous exchanges

### Error Handling
- **Timeout**: 30-second timeout for AI requests
- **Retry Logic**: Single retry on network errors
- **Fallback**: Mock service for development/testing

---

## 5. AI Provider Configuration

### Puter AI
- **Endpoint**: Configurable via `PUTER_AI_URL` environment variable
- **Default**: `https://api.puter.ai/v1/chat/completions`
- **Model**: `gpt-3.5-turbo` (default, configurable)
- **Authentication**: Optional API key via `PUTER_AI_KEY`

### Mock AI (Fallback)
- **Purpose**: Development and testing
- **Behavior**: Simple keyword matching
- **Response**: Mock responses with clear indicators

### Switching Providers
The AI service interface allows easy swapping:
```go
type AIService interface {
    AnswerQuestion(context, question string, history []string) (string, bool, error)
    GenerateSummary(text string) (string, []string, []string, error)
}
```

---

## 6. Prompt Optimization Tips

### For Better Accuracy
1. **Increase Context**: Include more relevant chunks (up to 5)
2. **Refine Chunks**: Improve chunking algorithm for better context preservation
3. **History Management**: Keep conversation history concise but relevant

### For Better Performance
1. **Reduce Context**: Limit chunks to top 3 for faster responses
2. **Truncate Text**: Limit document text in summary prompts
3. **Cache Responses**: Cache common questions (future enhancement)

### For Better User Experience
1. **Clear Instructions**: Explicit "not found" messaging
2. **Structured Output**: Consistent format for parsing
3. **Error Messages**: User-friendly error handling

---

## 7. Testing Prompts

### Test Cases

1. **Direct Question**
   - Question: "What is the main topic?"
   - Expected: Direct answer from document

2. **Not Found Question**
   - Question: "What is the author's email?"
   - Expected: "I cannot find this information in the document."

3. **Follow-up Question**
   - Question 1: "What is the company name?"
   - Question 2: "When was it founded?"
   - Expected: Uses conversation history for context

4. **Summary Request**
   - Document: 10-page PDF
   - Expected: Structured summary with takeaways and topics

---

## 8. Future Enhancements

### Planned Improvements
1. **Streaming Responses**: Real-time answer streaming
2. **Citation Support**: Include chunk/page references in answers
3. **Multi-language**: Support for non-English documents
4. **Custom Prompts**: User-configurable prompt templates
5. **Prompt Versioning**: Track and version prompt changes

---

## 9. Environment Variables

```bash
# Puter AI Configuration
PUTER_AI_URL=https://api.puter.ai/v1/chat/completions
PUTER_AI_KEY=your_api_key_here  # Optional

# For Mock AI (development)
# Leave PUTER_AI_URL and PUTER_AI_KEY empty
```

---

## 10. Troubleshooting

### Common Issues

1. **AI Not Following Instructions**
   - **Solution**: Strengthen system message, add explicit examples

2. **Answers Include External Knowledge**
   - **Solution**: Add stronger guardrails, validate against context

3. **Summary Format Inconsistent**
   - **Solution**: Improve prompt structure, add format examples

4. **Slow Response Times**
   - **Solution**: Reduce context size, optimize chunk selection

---

**Document Owner**: Development Team  
**Last Updated**: 2024  
**Next Review**: Post-MVP Launch

