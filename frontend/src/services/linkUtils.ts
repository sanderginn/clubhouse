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
const INTERNAL_UPLOAD_URL_PATTERN =
  /(?:https?:\/\/[^\s<>"{}|\\^`[\]]*\/api\/v1\/uploads[^\s<>"{}|\\^`[\]]*|\/api\/v1\/uploads[^\s<>"{}|\\^`[\]]*)/gi;
const INTERNAL_UPLOAD_MATCH = /^(?:https?:\/\/[^/]+)?\/api\/v1\/uploads(?:\/|$)/i;
const TRAILING_PUNCTUATION = /[).,;:!?]+$/;

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

export function isInternalUploadUrl(value: string): boolean {
  if (!value) {
    return false;
  }
  return INTERNAL_UPLOAD_MATCH.test(value.trim());
}

export function stripInternalUploadUrls(value: string): string {
  if (!value) {
    return value;
  }
  return value.replace(INTERNAL_UPLOAD_URL_PATTERN, (match) => {
    const trailing = match.match(TRAILING_PUNCTUATION)?.[0] ?? '';
    return trailing;
  });
}

export function getImageLinkUrl(link?: Link): string | undefined {
  if (!link) {
    return undefined;
  }

  const metadata = link.metadata;
  const metadataType = typeof metadata?.type === 'string' ? metadata.type.toLowerCase() : '';
  const isImage =
    metadataType === 'image' || metadataType.startsWith('image/') || looksLikeImageUrl(link.url);
  if (!isImage) {
    return undefined;
  }

  return metadata?.image ?? link.url;
}
