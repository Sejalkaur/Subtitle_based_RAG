import React from 'react';

export default function App() {
  return (
    <div style={{ fontFamily: 'system-ui, sans-serif', margin: '2rem' }}>
      <h1>Subtitle RAG — Web</h1>
      <p>Step 1 scaffold is running. Next steps will connect to the API and Qdrant.</p>
      <ul>
        <li>API health: <code>GET http://localhost:3001/health</code></li>
        <li>Web dev server: <code>http://localhost:5173</code></li>
      </ul>
    </div>
  );
}
