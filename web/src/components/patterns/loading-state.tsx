import {
  LoadingState as PublicLoadingState,
  zhCNPatternMessages,
  type LoadingStateProps as PublicLoadingStateProps,
} from "@lotusrain-net/backend-infrastructure-web/patterns";

export type LoadingStateProps = PublicLoadingStateProps;

export function LoadingState(props: LoadingStateProps) {
  return <PublicLoadingState messages={zhCNPatternMessages} {...props} />;
}
