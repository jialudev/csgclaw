import { useId, useState } from "react";
import { Link } from "react-router-dom";
import { Button } from "@/components/ui";
import { TaskStatusPill } from "@/components/business/TaskStatusPill";
import type { TranslateFn } from "@/models/conversations";
import { roomTaskExecutionEnded, roomTaskGroups } from "@/models/roomTasks";
import { pathForPane, WorkspacePaneTypes } from "@/models/routing";
import type { WorkspaceTask } from "@/models/tasks";
import styles from "./RoomTaskPlan.module.css";

type Props = {
  roomID: string;
  tasks: readonly WorkspaceTask[];
  loading: boolean;
  error: string;
  refreshing: boolean;
  onRefresh: () => void;
  t: TranslateFn;
};

export function RoomTaskPlan({ roomID, tasks, loading, error, refreshing, onRefresh, t }: Props) {
  const [expanded, setExpanded] = useState(true);
  const contentID = useId();
  const groups = roomTaskGroups(tasks, roomID);
  const children = groups.flatMap((group) => group.children);
  const completed = children.filter((task) => task.status === "completed").length;

  return (
    <section className={styles.panel} aria-label={t("roomPlanTitle")}>
      <div className={styles.toolbar}>
        <Button
          variant="tertiaryGray"
          size="sm"
          aria-expanded={expanded}
          aria-controls={contentID}
          onClick={() => setExpanded(!expanded)}
        >
          {t("roomPlanTitle")} {children.length ? t("roomPlanProgress", { completed, total: children.length }) : ""}
          <span aria-hidden="true">{expanded ? "▴" : "▾"}</span>
        </Button>
        <Button variant="tertiaryGray" size="sm" disabled={refreshing} onClick={onRefresh}>
          {t("tasksRefreshShort")}
        </Button>
      </div>
      {loading ? <p role="status">{t("roomPlanLoading")}</p> : null}
      {error ? (
        <p role="alert">
          {t("roomPlanLoadFailed")} {error}
        </p>
      ) : null}
      {!loading && !error && !groups.length ? <p>{t("roomPlanEmpty")}</p> : null}
      <div id={contentID} hidden={!expanded} className={styles.details}>
        {groups.map(({ task, children: steps }) => (
          <div key={task.id}>
            <div className={styles.taskHeading}>
              <Link to={pathForPane({ type: WorkspacePaneTypes.task, id: task.id })}>{task.title}</Link>
              <TaskStatusPill status={task.status} showFullLabel t={t} />
            </div>
            <p className={styles.muted}>
              {t("roomPlanCoordination")} · {task.id}
            </p>
            {task.plan_summary || task.body ? <p className={styles.preWrap}>{task.plan_summary || task.body}</p> : null}
            {task.error ? (
              <p role="alert" className={styles.preWrap}>
                {task.error}
              </p>
            ) : null}
            {!steps.length ? (
              <p>{t(roomTaskExecutionEnded(task) ? "roomPlanNoChildren" : "roomPlanWaiting")}</p>
            ) : (
              <>
                <p className={styles.muted}>{t("roomPlanSerial")}</p>
                <ol className={styles.steps}>
                  {steps.map((child) => (
                    <li key={child.id}>
                      <div className={styles.taskHeading}>
                        <Link to={pathForPane({ type: WorkspacePaneTypes.task, id: child.id })}>{child.title}</Link>
                        <TaskStatusPill status={child.status} showFullLabel t={t} />
                      </div>
                      <p className={styles.muted}>
                        {t("taskAssigneeLabel")}:{" "}
                        {child.assigned_to_agent_name || child.assigned_to || t("taskAssigneeUnassigned")}
                      </p>
                      {child.depends_on.length ? (
                        <p className={styles.muted}>
                          {t("taskDependsOnLabel")}:{" "}
                          {child.depends_on.map((id) => steps.find((step) => step.id === id)?.title || id).join(" / ")}
                        </p>
                      ) : null}
                      {child.body ? <p className={styles.preWrap}>{child.body}</p> : null}
                      {child.result ? (
                        <p className={styles.preWrap}>
                          {t("taskResultLabel")}: {child.result}
                        </p>
                      ) : null}
                      {child.error ? (
                        <p role="alert" className={styles.preWrap}>
                          {child.error}
                        </p>
                      ) : null}
                    </li>
                  ))}
                </ol>
              </>
            )}
            {roomTaskExecutionEnded(task) ? (
              <div className={styles.summary}>
                <strong>{t("roomTaskSummary")}</strong>
                <p>{task.goal_outcome ? t("roomTaskOutcome." + task.goal_outcome) : t("roomTaskAwaitingSummary")}</p>
                {task.report ? <p className={styles.preWrap}>{task.report}</p> : null}
                {task.report_status === "pending" ? <p>{t("roomTaskReportPending")}</p> : null}
              </div>
            ) : null}
          </div>
        ))}
      </div>
    </section>
  );
}
