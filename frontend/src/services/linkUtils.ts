import type { Link } from '../stores/postStore';

const IMAGE_EXTENSION_PATTERN = /\.(jpg|jpeg|png|gif|webp|bmp|svg|avif|tif|tiff)(?:$|[?#&])/i;
const IMAGE_FORMAT_PATTERN = /(?:format|fm|ext|type)=([a-z0-9]+)/i;
const IMAGE_FORMATS = new Set([
  'jpg',
  'jpeg',
  'png',
  'gif',
  'webp',
  'bmp',
  'svg',
  'avif',
  'tif',
  'tiff',
  'image',
]);

export function looksLikeImageUrl(value: string): boolean {
  if (!value) {
    return false;
  }

  if (IMAGE_EXTENSION_PATTERN.test(value)) {
    return true;
  }

  const match = value.toLowerCase().match(IMAGE_FORMAT_PATTERN);
  if (!match) {
    return false;
  }

  return IMAGE_FORMATS.has(match[1]);
}

export function getImageLinkUrl(link?: Link): string | undefined {
  if (!link) {
    return undefined;
  }

  const metadata = link.metadata;
  const isImage = metadata?.type === 'image' || looksLikeImageUrl(link.url);
  if (!isImage) {
    return undefined;
  }

  return metadata?.image ?? link.url;
}
