//logic for chunking
// Chunker: group subtitle cues into time windows with overlap
//
// Input: array of normalized cues
//   { fileId, index, startMs, endMs, text }
// Output: array of chunks
//   { chunkId, fileId, startMs, endMs, text, lineIndices }

function formatChunkId(fileId, startMs, endMs) {
  return `${fileId}#${startMs}-${endMs}`;
}

/**
 * Chunk cues by sliding time windows
 * @param {Array} cues - parsed cues
 * @param {object} opts
 * @param {number} opts.windowMs - window size (default 60000 = 60s)
 * @param {number} opts.overlapMs - overlap between windows (default 10000 = 10s)
 */
export function chunkCues(cues, { windowMs = 60000, overlapMs = 10000 } = {}) {
  if (!Array.isArray(cues) || cues.length === 0) return [];

  // sort by start time
  cues.sort((a, b) => a.startMs - b.startMs);

  const chunks = [];
  let winStart = cues[0].startMs;

  //O(n^2) complexity
  while (winStart <= cues[cues.length - 1].endMs) {
    const winEnd = winStart + windowMs;

    // select cues inside this window
    const inside = cues.filter((c) => c.startMs < winEnd && c.endMs > winStart);

    if (inside.length > 0) {
      const text = inside.map((c) => c.text).join(' ');
      chunks.push({
        chunkId: formatChunkId(cues[0].fileId, winStart, winEnd),
        courseId: inside[0].courseId,
        fileId: cues[0].fileId,
        startMs: inside[0].startMs,
        endMs: inside[inside.length - 1].endMs,
        text,
        lineIndices: inside.map((c) => c.index),
      });
    }

    // slide window forward (window - overlap)
    winStart += windowMs - overlapMs;
  }

  return chunks;
}
