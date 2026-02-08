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

export interface ChatResponse {
  response: string;
  session_id: string;
  answer_found: boolean;
  relevant_chunks?: string[];
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

export const sendMessage = async (sessionId: string, message: string): Promise<ChatResponse> => {
  const response = await api.post<ChatResponse>('/chat/message', {
    session_id: sessionId,
    message,
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

export default api;

