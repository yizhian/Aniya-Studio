interface Props {
  url: string;
}

export function PdfViewer({ url }: Props) {
  return (
    <iframe
      src={url}
      className="w-full h-full"
      title="PDF Viewer"
    />
  );
}
