interface Props {
  url: string;
  alt: string;
}

export function ImageViewer({ url, alt }: Props) {
  return (
    <div className="flex items-center justify-center p-4 h-full">
      <img src={url} alt={alt} className="max-w-full max-h-full object-contain rounded" />
    </div>
  );
}
