/**
 * Example: How to integrate AgentLifecycleControls into daemon UI
 * 
 * This shows the minimal integration pattern for the lifecycle control entry point.
 * Adapt to your specific agent display component (row, card, modal, etc.).
 */

import AgentLifecycleControls from "./AgentLifecycleControls";
import { useControlAgent, useSendAgentDirectMessage } from "@/hooks/useAgentLifecycleControl";

interface AgentRowProps {
  agentId: string;
  agentName: string;
  isOnline: boolean;
  operationState?: string;
}

export function AgentRowWithControls({
  agentId,
  agentName,
  isOnline,
  operationState = "unsupported",
}: AgentRowProps) {
  const { mutateAsync: controlAgent } = useControlAgent();
  const { mutateAsync: sendMessage } = useSendAgentDirectMessage();

  const handleTerminate = async (id: string, requestId: string) => {
    await controlAgent({
      agentId: id,
      action: "terminate",
      request_id: requestId,
    });
  };

  const handleRestart = async (id: string, requestId: string) => {
    await controlAgent({
      agentId: id,
      action: "restart",
      request_id: requestId,
    });
  };

  const handleResetSession = async (id: string, requestId: string) => {
    await controlAgent({
      agentId: id,
      action: "restart_reset_session",
      request_id: requestId,
    });
  };

  const handleFullReset = async (id: string, requestId: string) => {
    await controlAgent({
      agentId: id,
      action: "restart_full_reset",
      request_id: requestId,
    });
  };

  const handleDirectMessage = (id: string) => {
    // Navigate to DM view or open message modal
    // Example: router.push(`/dm/@${id}`)
    console.log(`Open DM with ${id}`);
  };

  return (
    <div className="flex items-center justify-between p-4 border rounded">
      <div>
        <div className="font-semibold">{agentName}</div>
        <div className="text-sm text-gray-500">{agentId}</div>
      </div>

      <AgentLifecycleControls
        agentId={agentId}
        agentName={agentName}
        isSupported={isOnline}
        isDisabled={!isOnline}
        operationState={operationState as any}
        onTerminate={handleTerminate}
        onRestart={handleRestart}
        onResetSession={handleResetSession}
        onFullReset={handleFullReset}
        onDirectMessage={handleDirectMessage}
      />
    </div>
  );
}
