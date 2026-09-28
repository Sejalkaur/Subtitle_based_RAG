// VTT → normalized JSON
// Shape: { fileId, index, startMs, endMs, text }

import webvtt from 'node-webvtt';

/** Convert HH:MM:SS.mmm or MM:SS.mmm (string) → milliseconds */
function hhmmssToMs(str) {
  if (!str) return 0;
  const s = String(str).replace(',', '.'); // unify decimal
  const parts = s.split(':');
  let h = 0,
    m = 0,
    sec = 0;
  if (parts.length === 3) {
    h = Number(parts[0]) || 0;
    m = Number(parts[1]) || 0;
    sec = Number(parts[2]) || 0;
  } else if (parts.length === 2) {
    m = Number(parts[0]) || 0;
    sec = Number(parts[1]) || 0;
  } else {
    sec = Number(s) || 0;
  }
  return Math.round((h * 3600 + m * 60 + sec) * 1000);
}

/** Light cleanup: strip tags, collapse whitespace, normalize ellipses */
function cleanText(t) {
  if (!t) return '';
  return t
    .replace(/<[^>]+>/g, ' ') // remove HTML tags (rare in VTT)
    .replace(/[\r\n]+/g, ' ') // newlines → space
    .replace(/\s+/g, ' ') // collapse spaces
    .replace(/…/g, '...') // normalize ellipses
    .trim();
}

/** Parse .vtt content → normalized array */
export function parseVttToNormalized(vttText, fileId, courseId = 'default') {
  const { cues } = webvtt.parse(vttText);
  const out = [];
  for (let i = 0; i < cues.length; i++) {
    const c = cues[i];
    const startMs =
      typeof c.start === 'number' ? Math.round(c.start * 1000) : hhmmssToMs(String(c.start));
    const endMs = typeof c.end === 'number' ? Math.round(c.end * 1000) : hhmmssToMs(String(c.end));
    out.push({
      courseId, // NEW
      fileId,
      index: i + 1,
      startMs,
      endMs,
      text: cleanText(c.text),
    });
  }
  return out;
}
