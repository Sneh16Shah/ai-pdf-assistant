import { useState } from 'react';
import ReactMarkdown from 'react-markdown';
import { generateSummary, SummaryResponse } from '../services/api';

interface SummaryViewProps {
  sessionId: string;
}

export default function SummaryView({ sessionId }: SummaryViewProps) {
  const [summary, setSummary] = useState<SummaryResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleGenerate = async () => {
    setLoading(true);
    setError(null);

    try {
      const response = await generateSummary(sessionId);
      setSummary(response);
    } catch (err: any) {
      setError(err.message || 'Failed to generate summary');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="w-full max-w-4xl mx-auto p-6">
      <div className="bg-white rounded-lg shadow-lg p-6">
        <div className="flex justify-between items-center mb-4">
          <h2 className="text-2xl font-bold text-gray-800">PDF Summary</h2>
          <button
            onClick={handleGenerate}
            disabled={loading}
            className="px-4 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700 disabled:opacity-50"
          >
            {loading ? 'Generating...' : 'Generate Summary'}
          </button>
        </div>

        {error && (
          <div className="mb-4 p-3 bg-red-100 border border-red-400 text-red-700 rounded">
            {error}
          </div>
        )}

        {summary && (
          <div className="space-y-6">
            {/* Main Summary */}
            <div>
              <h3 className="text-lg font-semibold text-gray-700 mb-2">Summary</h3>
              <div className="bg-gray-50 rounded-lg p-4">
                <div className="prose max-w-none">
                  <ReactMarkdown>{summary.summary}</ReactMarkdown>
                </div>
              </div>
            </div>

            {/* Key Takeaways */}
            {summary.key_takeaways && summary.key_takeaways.length > 0 && (
              <div>
                <h3 className="text-lg font-semibold text-gray-700 mb-2">Key Takeaways</h3>
                <ul className="list-disc list-inside space-y-2 bg-gray-50 rounded-lg p-4">
                  {summary.key_takeaways.map((takeaway, index) => (
                    <li key={index} className="text-gray-800">{takeaway}</li>
                  ))}
                </ul>
              </div>
            )}

            {/* Main Topics */}
            {summary.main_topics && summary.main_topics.length > 0 && (
              <div>
                <h3 className="text-lg font-semibold text-gray-700 mb-2">Main Topics</h3>
                <div className="flex flex-wrap gap-2">
                  {summary.main_topics.map((topic, index) => (
                    <span
                      key={index}
                      className="px-3 py-1 bg-blue-100 text-blue-800 rounded-full text-sm"
                    >
                      {topic}
                    </span>
                  ))}
                </div>
              </div>
            )}
          </div>
        )}

        {!summary && !loading && (
          <div className="text-center text-gray-500 py-8">
            <p>Click "Generate Summary" to create a summary of your PDF</p>
          </div>
        )}
      </div>
    </div>
  );
}
