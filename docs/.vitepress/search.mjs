// Use the same Chinese word boundaries while indexing and searching.
export function tokenize(text) {
  const segmenter = new Intl.Segmenter('zh-CN', { granularity: 'word' });
  return Array.from(segmenter.segment(text), part => part.isWordLike ? part.segment.toLowerCase() : '')
    .filter(Boolean);
}
