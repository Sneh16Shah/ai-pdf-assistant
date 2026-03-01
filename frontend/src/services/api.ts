import axios from 'axios';

const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8081/api/v1';

const api = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json',
  },
});

export interface UploadResponse {
  document_id: string;
  session_id: string;
  filename: string;
  pages: number;
  chunks: number;
  message: string;
}

export interface Citation {
  page: number;
  text: string;
  document_id?: string;
  filename?: string;
}

export interface SessionDocument {
  id: string;
  filename: string;
  pages: number;
}

export interface SessionDocumentsResponse {
  documents: SessionDocument[];
  count: number;
}

export interface ChatResponse {
  response: string;
  session_id: string;
  answer_found: boolean;
  relevant_chunks?: string[];
  citations?: Citation[];
  image_base64?: string;
  image_mime_type?: string;
}

export interface ChatMessage {
  id?: string;
  role: 'user' | 'assistant';
  content: string;
  timestamp?: number;
}

export interface SummaryResponse {
  summary: string;
  key_takeaways: string[];
  main_topics: string[];
}

export const uploadPDF = async (file: File): Promise<UploadResponse> => {
  const formData = new FormData();
  formData.append('pdf', file);

  const response = await api.post<UploadResponse>('/pdf/upload', formData, {
    headers: {
      'Content-Type': 'multipart/form-data',
    },
  });

  return response.data;
};

export const addPDFToSession = async (sessionId: string, file: File): Promise<UploadResponse> => {
  const formData = new FormData();
  formData.append('pdf', file);

  const response = await api.post<UploadResponse>(`/pdf/session/${sessionId}/add`, formData, {
    headers: {
      'Content-Type': 'multipart/form-data',
    },
  });

  return response.data;
};

// Re-hydrate an existing session's in-memory state (after backend restart)
// without creating a new session in the database.
export const rehydrateSession = async (sessionId: string, file: File): Promise<UploadResponse> => {
  const formData = new FormData();
  formData.append('pdf', file);
  formData.append('session_id', sessionId);

  const response = await api.post<UploadResponse>('/pdf/rehydrate', formData, {
    headers: {
      'Content-Type': 'multipart/form-data',
    },
  });

  return response.data;
};

export const getSessionDocuments = async (sessionId: string): Promise<SessionDocument[]> => {
  const response = await api.get<SessionDocumentsResponse>(`/pdf/session/${sessionId}/documents`);
  return response.data.documents;
};

export const deleteDocument = async (sessionId: string, documentId: string): Promise<void> => {
  await api.delete(`/pdf/document/${documentId}?session_id=${sessionId}`);
};

// Download the actual PDF file for a document (returns blob URL)
export const downloadDocumentPDF = async (documentId: string): Promise<string> => {
  const response = await api.get(`/pdf/document/${documentId}/download`, {
    responseType: 'blob',
  });
  return URL.createObjectURL(response.data);
};

export const sendMessage = async (sessionId: string, message: string, pageNumber?: number): Promise<ChatResponse> => {
  const response = await api.post<ChatResponse>('/chat/message', {
    session_id: sessionId,
    message,
    ...(pageNumber && { page_number: pageNumber }),
  });

  return response.data;
};

export const getChatHistory = async (sessionId: string): Promise<ChatMessage[]> => {
  const response = await api.get(`/chat/history/${sessionId}`);
  return response.data.messages || [];
};

export const generateSummary = async (sessionId: string): Promise<SummaryResponse> => {
  const response = await api.post<SummaryResponse>('/pdf/summary', {
    session_id: sessionId,
  });

  return response.data;
};

export const clearSession = async (sessionId: string): Promise<void> => {
  await api.delete(`/chat/session/${sessionId}`);
};

export interface StreamCallbacks {
  onToken: (token: string) => void;
  onDone: (response: ChatResponse) => void;
  onError: (error: string) => void;
}

export const streamMessage = async (
  sessionId: string,
  message: string,
  callbacks: StreamCallbacks
): Promise<void> => {
  try {
    const token = localStorage.getItem('auth_token');
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    };
    if (token) {
      headers['Authorization'] = `Bearer ${token}`;
    }

    const response = await fetch(`${API_BASE_URL}/chat/stream`, {
      method: 'POST',
      headers,
      body: JSON.stringify({
        session_id: sessionId,
        message,
      }),
    });

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    const reader = response.body?.getReader();
    if (!reader) {
      throw new Error('No reader available');
    }

    const decoder = new TextDecoder();
    let buffer = '';

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split('\n');
      buffer = lines.pop() || '';

      for (const line of lines) {
        if (line.startsWith('event: ')) {
          // Event type line, skip to data line
          continue;
        }
        if (line.startsWith('data: ')) {
          const data = line.slice(6);
          try {
            const parsed = JSON.parse(data);
            // Check for different event types based on content
            if (parsed.content !== undefined) {
              callbacks.onToken(parsed.content);
            } else if (parsed.response !== undefined) {
              callbacks.onDone({
                response: parsed.response,
                session_id: parsed.session_id,
                answer_found: parsed.answer_found,
                citations: parsed.citations,
              });
            } else if (parsed.message !== undefined) {
              callbacks.onError(parsed.message);
            }
          } catch {
            // Skip malformed JSON
          }
        }
      }
    }
  } catch (error: unknown) {
    const errorMessage = error instanceof Error ? error.message : 'Stream error';
    callbacks.onError(errorMessage);
  }
};
// User session types and API
export interface UserSession {
  id: string;
  user_id: string;
  title: string;
  created_at: string;
  last_activity: string;
  documents?: {
    id: string;
    filename: string;
    pages: number;
    uploaded_at: string;
  }[];
}

export const getUserSessions = async (): Promise<UserSession[]> => {
  const response = await api.get('/user/sessions');
  return response.data.sessions;
};

export const deleteUserSession = async (sessionId: string): Promise<void> => {
  await api.delete(`/user/sessions/${sessionId}`);
};

export const getSessionMessages = async (sessionId: string) => {
  const response = await api.get(`/user/sessions/${sessionId}/messages`);
  return response.data.messages;
};

export default api;

// ============ Smart Mode API (token-optimized) ============

export interface SmartTokenStats {
  raw_tokens: number;
  after_preprocessing: number;
  after_retrieval: number;
  final_tokens: number;
  savings_percent: number;
  boilerplate_removed: number;
  chunks_original: number;
  chunks_after_dedup: number;
  chunks_used: number;
  textrank_summary_len: number;
}

export interface SmartChatResponse extends ChatResponse {
  token_stats?: SmartTokenStats;
}

export interface SmartSessionStats {
  total_queries: number;
  total_raw_tokens: number;
  total_final_tokens: number;
  total_saved_tokens: number;
  avg_savings_percent: number;
}

export const sendSmartMessage = async (
  sessionId: string,
  message: string,
  pageNumber?: number
): Promise<SmartChatResponse> => {
  const response = await api.post<SmartChatResponse>('/smart/message', {
    session_id: sessionId,
    message,
    ...(pageNumber && { page_number: pageNumber }),
  });
  return response.data;
};

export const getSmartStats = async (sessionId: string): Promise<SmartSessionStats> => {
  const response = await api.get(`/smart/stats/${sessionId}`);
  return response.data.stats;
};
