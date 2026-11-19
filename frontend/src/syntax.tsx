import { Fragment, type ReactNode } from "react";
import Prism from "prismjs";
import "prismjs/components/prism-yaml";
import "prismjs/components/prism-properties";
import "prismjs/components/prism-json";
import "prismjs/components/prism-bash";
import "prismjs/components/prism-powershell";
import "prismjs/components/prism-go";

const grammars: Record<string, string> = {
  yaml: "yaml",
  yml: "yaml",
  properties: "properties",
  json: "json",
  js: "javascript",
  css: "css",
  xml: "markup",
  html: "markup",
  sh: "bash",
  bash: "bash",
  ps1: "powershell",
  go: "go",
};

function renderToken(token: string | Prism.Token, index: number): ReactNode {
  if (typeof token === "string") return <Fragment key={index}>{token}</Fragment>;
  const content =
    typeof token.content === "string"
      ? token.content
      : (Array.isArray(token.content) ? token.content : [token.content]).map(renderToken);
  return (
    <span className={`syntax-${token.type}`} key={index}>
      {content}
    </span>
  );
}

export default function Syntax({ text, path }: { text: string; path: string }) {
  const grammar = Prism.languages[grammars[path.toLowerCase().split(".").at(-1) || ""]];
  return grammar ? Prism.tokenize(text, grammar).map(renderToken) : text;
}
