import {
  ErrorState as PublicErrorState,
  zhCNPatternMessages,
  type ErrorStateProps as PublicErrorStateProps,
} from "@lotusrain-net/backend-infrastructure-web/patterns";

export type ErrorStateProps = PublicErrorStateProps;

export function ErrorState(props: ErrorStateProps) {
  return <PublicErrorState messages={zhCNPatternMessages} {...props} />;
}
