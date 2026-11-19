import { Fragment, lazy, Suspense } from "react";

const Syntax = lazy(() => import("./syntax"));

export function Code({
  text,
  path = "",
  id,
  testId,
}: {
  text: string;
  path?: string;
  id?: string;
  testId?: string;
}) {
  return (
    <pre className="code" id={id} data-testid={testId} tabIndex={0}>
      <code>
        {path && text.length < 100_000 ? (
          <Suspense fallback={<Fragment>{text}</Fragment>}>
            <Syntax text={text} path={path} />
          </Suspense>
        ) : (
          text
        )}
      </code>
    </pre>
  );
}
