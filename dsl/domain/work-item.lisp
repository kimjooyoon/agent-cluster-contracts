;;;; WorkItem — the project's minimum domain concept. Represents one unit of
;;;; work an agent will pick up. Lifecycle and events arrive in later slices.

(in-package #:agent-cluster.dsl)

(defaggregate work-item
  (id    :type string :required t)
  (title :type string :required t)
  (state :type string :required t))
