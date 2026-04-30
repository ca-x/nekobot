/**
 * Integration: Direct Agent Message UI/API Wiring
 *
 * This shows how to integrate DirectAgentMessageDialog into agent display components.
 * Reuses dm:@agent_id target and SendMessage/SendAgentDirectMessage semantics.
 */

import { useState } from "react";
import DirectAgentMessageDialog from "./DirectAgentMessageDialog";
import { useSendAgentDirectMessage } from "@/hooks/useSendAgentDirectMessage";

interface AgentWithDirectMessageProps {
  agentId: string;
  agentName: string;
}

export function AgentWithDirectMessage({
  agentId,
  agentName,
}: AgentWithDirectMessageProps) {
  const [dmOpen, setDmOpen] = useState(false);
  const { mutateAsync: sendDirectMessage } = useSendAgentDirectMessage();

  const handleOpenDirectMessage = () => {
    setDmOpen(true);
  };

  const handleSendMessage = async (
    id: string,
    content: string,
    requestId: string
  ) => {
    await sendDirectMessage({
      agentId: id,
      content,
      request_id: requestId,
    });
  };

  return (
    <>
      <button
        onClick={handleOpenDirectMessage}
        className="px-3 py-1 text-sm rounded hover:bg-gray-100"
      >
        Message
      </button>

      <DirectAgentMessageDialog
        agentId={agentId}
        agentName={agentName}
        open={dmOpen}
        onOpenChange={setDmOpen}
        onSendMessage={handleSendMessage}
      />
    </>
  );
}

/**
 * Example: Minimal integration into agent row
 *
 * Usage in agent list/table:
 *
 * <div className="flex items-center justify-between">
 *   <div>
 *     <div className="font-semibold">{agent.name}</div>
 *     <div className="text-sm text-gray-500">{agent.id}</div>
 *   </div>
 *   <AgentWithDirectMessage agentId={agent.id} agentName={agent.name} />
 * </div>
 */

