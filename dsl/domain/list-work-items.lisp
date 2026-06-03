;;;; list-work-items — the single read query in this slice.
;;;;
;;;; Returns the full WorkItem list. The wire-name "workItems" is the
;;;; identifier the GraphQL endpoint matches; backend and frontend never
;;;; hand-type this string (constraint C-006).

(in-package #:agent-cluster.dsl)

(defquery list-work-items
  :wire-name "workItems"
  :returns-list work-item)
