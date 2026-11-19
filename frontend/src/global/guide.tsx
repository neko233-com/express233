import { useSearchParams } from "react-router";
import { useResource } from "../api";
import { Code } from "../code";
import { Card, PageHeader, ResourceNotice } from "../ui";

interface Topic {
  id: string;
  title: string;
  summary: string;
  content?: string;
}
export function Guide() {
  const [params, setParams] = useSearchParams();
  const index = useResource<{ topics: Topic[]; title: string }>("/api/agent/guide");
  const topics = index.data?.topics ?? [];
  const selected = topics.find((topic) => topic.id === params.get("topic")) ?? topics[0];
  const page = useResource<Topic>(
    selected && "/api/agent/guide/" + encodeURIComponent(selected.id),
  );
  return (
    <div className="section-stack" data-testid="guide-panel">
      <PageHeader title="文档目录" description="版本交付、CI 接入与 Agent 运维。">
        <a className="btn btn-secondary" href="/docs/" target="_blank" rel="noreferrer">
          API 文档
        </a>
        <a
          className="btn btn-secondary"
          href="https://neko233-com.github.io/express233/"
          target="_blank"
          rel="noreferrer"
        >
          使用文档
        </a>
      </PageHeader>
      <ResourceNotice loading={index.loading} error={index.error} retry={index.refresh} />
      {topics.length > 0 && (
        <div className="split-versions">
          <Card title="接入指南">
            <div className="option-list" data-testid="guide-topic-list">
              {topics.map((topic) => (
                <button
                  type="button"
                  className={`option-row ${topic.id === selected?.id ? "selected" : ""}`}
                  aria-label={topic.title}
                  aria-current={topic.id === selected?.id ? "page" : undefined}
                  key={topic.id}
                  onClick={() => setParams({ topic: topic.id })}
                >
                  <strong>{topic.title}</strong>
                  <small>{topic.summary}</small>
                </button>
              ))}
            </div>
          </Card>
          <Card title={selected?.title} description={selected?.summary}>
            <div data-testid="guide-content">
              <ResourceNotice loading={page.loading} error={page.error} retry={page.refresh} />
              {page.data?.content && (
                <div className="guide-body">
                  <Code text={page.data.content} />
                </div>
              )}
            </div>
          </Card>
        </div>
      )}
    </div>
  );
}
