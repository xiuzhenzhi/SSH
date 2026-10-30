export const pageIDs = ['dashboard','servers','events','alerts','channels','rules','ip-allowlist','geoip','login-protection','notifications','audit-logs','users'] as const;
export type PageID = typeof pageIDs[number];

export function pageFromPath(path: string): PageID {
  const id = path.replace(/\/+$/, '').slice(1);
  return pageIDs.includes(id as PageID) ? id as PageID : 'dashboard';
}

export function pathForPage(id: string): string {
  return '/' + (pageIDs.includes(id as PageID) ? id : 'dashboard');
}
