import {
  createHash
} from 'node:crypto';

export async function publicSource(repo, commit, file) {
  const [owner, name] = repo.split('/');
  const path = file.path.split('/').map(encodeURIComponent).join('/');
  const pagesRoot = name.toLowerCase() === `${owner.toLowerCase()}.github.io` ? '' : `${encodeURIComponent(name)}/`;
  const urls = [
    `https://${owner.toLowerCase()}.github.io/${pagesRoot}${path}`,
    `https://raw.githubusercontent.com/${repo}/${commit}/${path}`,
  ];
  for (const url of urls) {
    try {
      const response = await fetch(url, {
        signal: AbortSignal.timeout(10000),
        redirect: 'error'
      });
      if (!response.ok || !response.body) {
        await response.body?.cancel();
        continue;
      }
      const chunks = [];
      let size = 0;
      for await (const chunk of response.body) {
        size += chunk.length;
        if (size > 200000) break;
        chunks.push(chunk);
      }
      if (size > 200000) continue;
      const bytes = Buffer.concat(chunks);
      const hash = createHash('sha1').update(`blob ${bytes.length}\0`).update(bytes).digest('hex');
      if (hash === file.sha) return bytes.toString('utf8');
    } catch {
      // Public mirrors are optional; the authenticated blob API is the fallback.
    }
  }
  return null;
}
