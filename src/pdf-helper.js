// AskMyPDF — PDF Helper Script (Page Context)
// Injected into the main page context to access the PDF embed's postMessage API.
// Chrome's PDF viewer embed supports: embed.postMessage({type: 'getSelectedText'}, '*')

(function () {
    'use strict';

    // Listen for requests from the content script
    window.addEventListener('message', (event) => {
        if (event.data && event.data.type === 'askmypdf-getSelectedText') {
            const embed = document.querySelector('embed[type="application/pdf"]');
            if (!embed || !embed.postMessage) {
                window.postMessage({
                    type: 'askmypdf-selectedTextReply',
                    selectedText: '',
                }, '*');
                return;
            }

            // Set up listener for the PDF viewer's reply
            function onReply(e) {
                if (e.data && e.data.type === 'getSelectedTextReply') {
                    window.removeEventListener('message', onReply);
                    window.postMessage({
                        type: 'askmypdf-selectedTextReply',
                        selectedText: e.data.selectedText || '',
                    }, '*');
                }
            }

            window.addEventListener('message', onReply);

            // Ask the PDF viewer for selected text
            try {
                embed.postMessage({ type: 'getSelectedText' }, '*');
            } catch (err) {
                window.removeEventListener('message', onReply);
                window.postMessage({
                    type: 'askmypdf-selectedTextReply',
                    selectedText: '',
                }, '*');
            }

            // Timeout fallback
            setTimeout(() => {
                window.removeEventListener('message', onReply);
            }, 1000);
        }
    });
})();
