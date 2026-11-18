"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type { ViamConnection } from "./lib/viamClient";
import {
  cancelOrder,
  clearSlotAssignment,
  getStatus,
  moveBottle,
  retrieveBottle,
  stockBottle,
  updateSlotAssignment,
} from "./lib/viamClient";
import { titleCase } from "./lib/display";
import { slotLabel } from "./lib/stocking";
import type { StockingPlan } from "./lib/stocking";
import styles from "./stocking-modal.module.css";

export type StockingModalProps = {
  conn: ViamConnection;
  plan: StockingPlan;
  onClose: () => void;
  onDone: () => void;
};

type Stage = "confirm" | "running" | "swap-pause" | "done" | "failed";

export function StockingModal({ conn, plan, onClose, onDone }: StockingModalProps) {
  const [stage, setStage] = useState<Stage>("confirm");
  const [error, setError] = useState<string | null>(null);
  const [currentStep, setCurrentStep] = useState<string | null>(null);
  const abortedRef = useRef(false);

  useEffect(() => {
    if (stage !== "running") return;
    let cancelled = false;
    const tick = async () => {
      try {
        const s = await getStatus(conn);
        if (!cancelled) setCurrentStep(s.current_step ?? null);
      } catch {
        // polling errors are non-fatal; main op resolves on its own
      }
    };
    tick();
    const t = setInterval(tick, 1000);
    return () => {
      cancelled = true;
      clearInterval(t);
    };
  }, [conn, stage]);

  const runPhaseOne = useCallback(async () => {
    abortedRef.current = false;
    setError(null);
    setStage("running");
    try {
      if (plan.action === "place") {
        await stockBottle(conn, plan.targetSlot, plan.ingredient!);
      } else if (plan.action === "move") {
        await moveBottle(conn, plan.sourceSlot!, plan.targetSlot);
      } else if (plan.action === "retrieve") {
        await retrieveBottle(conn, plan.targetSlot);
      } else {
        await retrieveBottle(conn, plan.targetSlot);
      }
    } catch (err) {
      if (abortedRef.current) {
        setError("Arm stopped. Check that the bottle is standing upright before trying again.");
      } else {
        setError(err instanceof Error ? err.message : String(err));
      }
      setStage("failed");
      return;
    }
    if (plan.action === "swap") {
      setStage("swap-pause");
    } else {
      setStage("done");
    }
  }, [conn, plan]);

  const runPhaseTwo = useCallback(async () => {
    abortedRef.current = false;
    setError(null);
    setStage("running");
    try {
      await stockBottle(conn, plan.targetSlot, plan.ingredient!);
    } catch (err) {
      if (abortedRef.current) {
        setError("Arm stopped. Check that the bottle is standing upright before trying again.");
      } else {
        setError(err instanceof Error ? err.message : String(err));
      }
      setStage("failed");
      return;
    }
    setStage("done");
  }, [conn, plan]);

  // runManualFull updates the inventory to match a human-performed move, with no arm motion.
  // Covers the full "Manually moved" path from the initial confirm, including swap (one update).
  const runManualFull = useCallback(async () => {
    setError(null);
    try {
      if (plan.action === "place") {
        await updateSlotAssignment(conn, plan.targetSlot, plan.ingredient!);
      } else if (plan.action === "move") {
        await clearSlotAssignment(conn, plan.sourceSlot!);
        await updateSlotAssignment(conn, plan.targetSlot, plan.ingredient!);
      } else if (plan.action === "retrieve") {
        await clearSlotAssignment(conn, plan.targetSlot);
      } else {
        // swap: slot assignment is replaced in a single update.
        await updateSlotAssignment(conn, plan.targetSlot, plan.ingredient!);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
      setStage("failed");
      return;
    }
    setStage("done");
  }, [conn, plan]);

  // runManualPhaseTwo is the swap-pause variant: the arm has already cleared the slot
  // and the operator will place the new bottle by hand instead of using the arm.
  const runManualPhaseTwo = useCallback(async () => {
    setError(null);
    try {
      await updateSlotAssignment(conn, plan.targetSlot, plan.ingredient!);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
      setStage("failed");
      return;
    }
    setStage("done");
  }, [conn, plan]);

  const stop = useCallback(async () => {
    abortedRef.current = true;
    try {
      await cancelOrder(conn);
    } catch {
      // the op's own error will carry whatever the cancel caused
    }
  }, [conn]);

  const close = useCallback(() => {
    if (stage === "running") return;
    if (stage === "done") onDone();
    onClose();
  }, [stage, onClose, onDone]);

  return (
    <div className={styles.overlay} onClick={stage === "running" ? undefined : close}>
      <div className={styles.modal} role="dialog" onClick={(e) => e.stopPropagation()}>
        {stage === "confirm" && (
          <ConfirmBody plan={plan} onConfirm={runPhaseOne} onManual={runManualFull} onClose={close} />
        )}
        {stage === "running" && <RunningBody plan={plan} currentStep={currentStep} onStop={stop} />}
        {stage === "swap-pause" && (
          <SwapPauseBody
            plan={plan}
            onContinue={runPhaseTwo}
            onManual={runManualPhaseTwo}
            onClose={close}
          />
        )}
        {stage === "done" && <DoneBody plan={plan} onDone={close} />}
        {stage === "failed" && <FailedBody error={error} onRetry={runPhaseOne} onClose={close} />}
      </div>
    </div>
  );
}

function ConfirmBody({
  plan,
  onConfirm,
  onManual,
  onClose,
}: {
  plan: StockingPlan;
  onConfirm: () => void;
  onManual: () => void;
  onClose: () => void;
}) {
  const t = confirmText(plan);
  return (
    <>
      <h2 className={styles.title}>{t.title}</h2>
      <p className={styles.lede}>{t.lede}</p>
      <Diagram from={t.from} to={t.to} fromFilled={t.fromFilled} />
      <ol className={styles.steps}>
        {t.steps.map((s, i) => (
          <li key={i}>{s}</li>
        ))}
      </ol>
      <Warn>The arm starts moving as soon as you confirm.</Warn>
      <div className={styles.actions}>
        <button type="button" className={styles.btnGhost} onClick={onClose}>
          Close
        </button>
        <button type="button" className={styles.btnGhost} onClick={onManual}>
          I moved it manually
        </button>
        <button type="button" className={styles.btnPrimary} onClick={onConfirm}>
          Confirm
        </button>
      </div>
    </>
  );
}

function RunningBody({
  plan,
  currentStep,
  onStop,
}: {
  plan: StockingPlan;
  currentStep: string | null;
  onStop: () => void;
}) {
  return (
    <>
      <h2 className={styles.title}>{currentStep ?? runningTitle(plan)}</h2>
      <div className={styles.progress} aria-label="arm moving" />
      <Warn>The arm is moving.</Warn>
      <div className={styles.actions}>
        <button type="button" className={styles.stopLink} onClick={onStop}>
          Stop arm
        </button>
      </div>
    </>
  );
}

function SwapPauseBody({
  plan,
  onContinue,
  onManual,
  onClose,
}: {
  plan: StockingPlan;
  onContinue: () => void;
  onManual: () => void;
  onClose: () => void;
}) {
  const newName = titleCase(plan.ingredient!);
  const oldName = titleCase(plan.removedIngredient!);
  return (
    <>
      <h2 className={styles.title}>Swap the bottles</h2>
      <p className={styles.lede}>{oldName} is on the loading spot.</p>
      <ol className={styles.steps}>
        <li>Take the {oldName} bottle off the loading spot.</li>
        <li>Put the {newName} bottle there, spout facing the arm.</li>
        <li>Press Continue.</li>
      </ol>
      <Warn>The arm starts moving as soon as you press Continue.</Warn>
      <div className={styles.actions}>
        <button type="button" className={styles.btnGhost} onClick={onClose}>
          Close
        </button>
        <button type="button" className={styles.btnGhost} onClick={onManual}>
          I moved it manually
        </button>
        <button type="button" className={styles.btnPrimary} onClick={onContinue}>
          Continue
        </button>
      </div>
    </>
  );
}

function DoneBody({ plan, onDone }: { plan: StockingPlan; onDone: () => void }) {
  return (
    <>
      <h2 className={styles.titleOk}>
        <span aria-hidden>✓</span> {doneText(plan)}
      </h2>
      <div className={styles.actions}>
        <button type="button" className={styles.btnPrimary} onClick={onDone}>
          Done
        </button>
      </div>
    </>
  );
}

function FailedBody({
  error,
  onRetry,
  onClose,
}: {
  error: string | null;
  onRetry: () => void;
  onClose: () => void;
}) {
  return (
    <>
      <h2 className={styles.titleFail}>Arm stopped</h2>
      <p className={styles.lede}>{error ?? "Unknown failure."}</p>
      <div className={styles.actions}>
        <button type="button" className={styles.btnGhost} onClick={onClose}>
          Close
        </button>
        <button type="button" className={styles.btnPrimary} onClick={onRetry}>
          Try again
        </button>
      </div>
    </>
  );
}

function Diagram({ from, to, fromFilled }: { from: string; to: string; fromFilled: boolean }) {
  return (
    <div className={styles.diagram}>
      <DiagramNode label={from} filled={fromFilled} />
      <span className={styles.arrow} aria-hidden>
        →
      </span>
      <DiagramNode label={to} filled={!fromFilled} />
    </div>
  );
}

function DiagramNode({ label, filled }: { label: string; filled: boolean }) {
  return (
    <div className={styles.node}>
      <div className={`${styles.nodeBox} ${filled ? styles.nodeBoxFilled : ""}`}>
        {filled && <span className={styles.bottle} aria-hidden />}
      </div>
      <span className={styles.nodeLabel}>{label}</span>
    </div>
  );
}

function Warn({ children }: { children: React.ReactNode }) {
  return (
    <div className={styles.warn}>
      <span aria-hidden>⚠</span>
      <span>
        <strong>Keep hands clear.</strong> {children}
      </span>
    </div>
  );
}

const LOADING_SPOT = "Loading spot";

function confirmText(plan: StockingPlan) {
  if (plan.action === "place") {
    const name = titleCase(plan.ingredient!);
    return {
      title: `Place ${name}?`,
      lede: "The arm is about to move the bottle to its correct spot.",
      from: LOADING_SPOT,
      to: slotLabel(plan.targetSlot),
      fromFilled: true,
      steps: [
        `Put the ${name} bottle on the loading spot, spout facing the arm.`,
        `Press Confirm. The arm moves it to ${slotLabel(plan.targetSlot)}.`,
      ],
    };
  }
  if (plan.action === "move") {
    const name = titleCase(plan.ingredient!);
    return {
      title: `Move ${name} to ${slotLabel(plan.targetSlot)}?`,
      lede: "The arm is about to move the bottle to its new spot.",
      from: slotLabel(plan.sourceSlot!),
      to: slotLabel(plan.targetSlot),
      fromFilled: true,
      steps: [`The arm moves ${name} from ${slotLabel(plan.sourceSlot!)} to ${slotLabel(plan.targetSlot)}.`],
    };
  }
  if (plan.action === "retrieve") {
    const name = titleCase(plan.removedIngredient!);
    return {
      title: `Take ${name} off the rack?`,
      lede: "The arm is about to retrieve the bottle.",
      from: slotLabel(plan.targetSlot),
      to: LOADING_SPOT,
      fromFilled: true,
      steps: [
        `The arm brings it from ${slotLabel(plan.targetSlot)} to the loading spot.`,
        "Pick it up once the arm has moved away.",
      ],
    };
  }
  const newName = titleCase(plan.ingredient!);
  const oldName = titleCase(plan.removedIngredient!);
  return {
    title: `Swap ${oldName} for ${newName}?`,
    lede: "The arm is about to retrieve the current bottle, then move the new one to its spot.",
    from: slotLabel(plan.targetSlot),
    to: LOADING_SPOT,
    fromFilled: true,
    steps: [
      `The arm brings ${oldName} to the loading spot.`,
      `You swap in the ${newName} bottle.`,
      `The arm moves ${newName} to ${slotLabel(plan.targetSlot)}.`,
    ],
  };
}

function runningTitle(plan: StockingPlan): string {
  if (plan.action === "place") {
    return `Moving ${titleCase(plan.ingredient!)} to ${slotLabel(plan.targetSlot)}`;
  }
  if (plan.action === "move") {
    return `Moving ${titleCase(plan.ingredient!)} to ${slotLabel(plan.targetSlot)}`;
  }
  if (plan.action === "retrieve") {
    return `Retrieving ${titleCase(plan.removedIngredient!)}`;
  }
  return `Moving ${titleCase(plan.ingredient!)} to ${slotLabel(plan.targetSlot)}`;
}

function doneText(plan: StockingPlan): string {
  if (plan.action === "retrieve") {
    return `${titleCase(plan.removedIngredient!)} is on the loading spot.`;
  }
  return `${titleCase(plan.ingredient!)} is in ${slotLabel(plan.targetSlot)}.`;
}
