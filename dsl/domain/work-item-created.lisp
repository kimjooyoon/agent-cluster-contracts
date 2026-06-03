;;;; WorkItemCreated — the single event emitted when a new WorkItem appears.
;;;; This is the one event surface required by decision 003 (first vertical
;;;; slice). All other events arrive with later decisions.

(in-package #:agent-cluster.dsl)

(defevent work-item-created
  (work-item-id :type string :required t)
  (title        :type string :required t))
