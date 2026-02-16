package usecases

// EstimateTokens provides a rough token count estimate for English text.
// ~4 characters per token for English text (OpenAI/Llama tokenization)
func EstimateTokens(text string) int {
	return len(text) / 4
}

// EstimateCharsFromTokens converts token count to approximate character count
func EstimateCharsFromTokens(tokens int) int {
	return tokens * 4
}
