import type { ImgHTMLAttributes } from "react";

type HtmlImageProps = Omit<ImgHTMLAttributes<HTMLImageElement>, "src"> & {
  src: string;
  unoptimized?: boolean;
};

export function HtmlImage({ unoptimized: _unoptimized, ...props }: HtmlImageProps) {
  return <img {...props} />;
}
