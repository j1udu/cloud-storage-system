// 把后端返回的字节数转换为适合表格展示的文件大小。
export function formatBytes(size: number) {
  if (!size) {
    return '-';
  }

  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let value = size;
  let index = 0;

  while (value >= 1024 && index < units.length - 1) {
    value /= 1024;
    index += 1;
  }

  return `${value.toFixed(value >= 10 || index === 0 ? 0 : 1)} ${units[index]}`;
}

// 将后端时间字符串格式化为中文日期时间，供文件列表展示。
export function formatDate(value: string) {
  if (!value) {
    return '-';
  }

  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value));
}
