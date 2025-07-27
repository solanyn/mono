import React from 'react'
import { marked } from 'marked';

interface MarkdownRendererProps {
  content: string;
  className?: string;
}

export default function MarkdownRenderer({ content, className = '' }: MarkdownRendererProps) {
  const htmlContent = marked(content);
  
  return (
    <div 
      className={`prose ${className}`}
      dangerouslySetInnerHTML={{ __html: htmlContent }} 
    />
  );
}