;;;; agent-cluster DSL core
;;;; ----------------------
;;;; Minimal DSL surface for the first vertical slice. Currently exports one
;;;; macro: DEFAGGREGATE. Adding any new macro requires a decision record per
;;;; the initial agreement.
;;;;
;;;; A DSL form does not emit IR directly — it pushes a plist describing the
;;;; definition onto *REGISTRY*. emit-ir.lisp walks the registry and writes
;;;; deterministic JSON IR.

(defpackage #:agent-cluster.dsl
  (:use #:cl)
  (:export #:*registry*
           #:defaggregate
           #:defevent))

(in-package #:agent-cluster.dsl)

(defvar *registry* nil
  "Reverse-chronological list of definitions made by DSL forms. Each entry is
a plist with at least :KIND, :NAME, and :SOURCE-FILE. emit-ir.lisp NREVERSEs
this to get load order before emitting.")

(defun %current-source-file ()
  "Truename of the file currently being LOADed, or NIL."
  (when *load-truename*
    (namestring *load-truename*)))

(defmacro defaggregate (name &body slots)
  "Declare an aggregate root.

NAME is a symbol; it becomes the aggregate's canonical name (kebab-cased on
emit). Each slot is (SLOT-NAME :type TYPE-KEYWORD :required BOOLEAN).

Example:
  (defaggregate work-item
    (id    :type string :required t)
    (title :type string :required t)
    (state :type string :required t))"
  (let ((src (%current-source-file)))
    `(push (list :kind :aggregate
                 :name ',name
                 :slots ',slots
                 :source-file ,src)
           *registry*)))

(defmacro defevent (name &body slots)
  "Declare a domain event.

Same slot syntax as DEFAGGREGATE. Event slots are always treated as required
(events are immutable snapshots); the :required key is accepted but not
enforced differently.

Example:
  (defevent work-item-created
    (work-item-id :type string)
    (title        :type string))"
  (let ((src (%current-source-file)))
    `(push (list :kind :event
                 :name ',name
                 :slots ',slots
                 :source-file ,src)
           *registry*)))
