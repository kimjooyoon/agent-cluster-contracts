;;;; Emit JSON IR from the DSL.
;;;; Run from the contracts/ repo root:   sbcl --script dsl/emit-ir.lisp
;;;;
;;;; Output: ir/domain/<name>.ir.json per registered aggregate, with the JSON
;;;; written deterministically (sorted keys, 2-space indent, LF newlines).
;;;; Each file records its source DSL path and a sha256 of that path so
;;;; tools/irdrift can detect hand-edits and re-generation drift.
;;;;
;;;; sha256 is computed by shelling out to /usr/bin/shasum -a 256 to avoid
;;;; pulling Ironclad (a third-party dep) without a decision record.

(load "dsl/core.lisp")

(in-package #:agent-cluster.dsl)

(setf *registry* nil)

;; Load every domain file in lexicographic order — sort makes the emit
;; deterministic across filesystems.
(dolist (file (sort (mapcar #'namestring (directory "dsl/domain/*.lisp"))
                    #'string<))
  (load file))

(setf *registry* (nreverse *registry*))

;; ---------------------------------------------------------------------------
;; JSON emitter: minimal, deterministic, no deps.
;; Object values are tagged (:OBJ (key . v) ...) and arrays (:ARR v ...).
;; ---------------------------------------------------------------------------

(defun emit-json-string (s stream)
  (write-char #\" stream)
  (loop for c across s do
       (case c
         (#\\ (write-string "\\\\" stream))
         (#\" (write-string "\\\"" stream))
         (#\Newline (write-string "\\n" stream))
         (#\Tab (write-string "\\t" stream))
         (#\Return (write-string "\\r" stream))
         (otherwise
          (if (< (char-code c) 32)
              (format stream "\\u~4,'0X" (char-code c))
              (write-char c stream)))))
  (write-char #\" stream))

(defun emit-indent (stream n)
  (loop repeat n do (write-char #\Space stream)))

(defun emit-json (value stream indent)
  (cond
    ((null value)         (write-string "null" stream))
    ((eq value t)         (write-string "true" stream))
    ((eq value 'false)    (write-string "false" stream))
    ((stringp value)      (emit-json-string value stream))
    ((numberp value)      (format stream "~A" value))
    ((and (consp value) (eq (car value) :obj))
     (emit-json-object (cdr value) stream indent))
    ((and (consp value) (eq (car value) :arr))
     (emit-json-array (cdr value) stream indent))
    ((symbolp value)
     (emit-json-string (kebab value) stream))
    (t (error "emit-json: cannot emit ~S" value))))

(defun emit-json-object (pairs stream indent)
  (cond
    ((null pairs) (write-string "{}" stream))
    (t
     (let* ((sorted (sort (copy-list pairs) #'string< :key #'car))
            (next   (+ indent 2)))
       (write-char #\{ stream)
       (write-char #\Newline stream)
       (loop for (pair . rest) on sorted do
            (emit-indent stream next)
            (emit-json-string (car pair) stream)
            (write-string ": " stream)
            (emit-json (cdr pair) stream next)
            (when rest (write-char #\, stream))
            (write-char #\Newline stream))
       (emit-indent stream indent)
       (write-char #\} stream)))))

(defun emit-json-array (items stream indent)
  (cond
    ((null items) (write-string "[]" stream))
    (t
     (let ((next (+ indent 2)))
       (write-char #\[ stream)
       (write-char #\Newline stream)
       (loop for (item . rest) on items do
            (emit-indent stream next)
            (emit-json item stream next)
            (when rest (write-char #\, stream))
            (write-char #\Newline stream))
       (emit-indent stream indent)
       (write-char #\] stream)))))

;; ---------------------------------------------------------------------------
;; Helpers
;; ---------------------------------------------------------------------------

(defun kebab (sym)
  (string-downcase (substitute #\- #\_ (symbol-name sym))))

(defun %try-sha256 (program args)
  (handler-case
      (let* ((proc (sb-ext:run-program program args :search t
                                                    :output :stream
                                                    :wait t)))
        (when (zerop (sb-ext:process-exit-code proc))
          (with-open-stream (s (sb-ext:process-output proc))
            (let ((line (read-line s nil nil)))
              (and line (>= (length line) 64) (subseq line 0 64))))))
    (error () nil)))

(defun sha256-of-file (path)
  "Compute sha256 hex of PATH using whichever standard tool is on PATH.
Tries sha256sum (Linux native) first, then shasum -a 256 (macOS native).
Both are stdlib-free and avoid pulling Ironclad without a decision record."
  (or (%try-sha256 "sha256sum" (list (namestring path)))
      (%try-sha256 "shasum"    (list "-a" "256" (namestring path)))
      (error "sha256 of ~A failed: no sha256sum or shasum on PATH" path)))

(defun rel-to-contracts-root (abs)
  "Return ABS relative to the contracts/ root by stripping everything up to
and including '/contracts/'. Falls back to ABS if the marker is absent."
  (let* ((p (namestring abs))
         (marker "/contracts/")
         (pos (search marker p)))
    (if pos
        (subseq p (+ pos (length marker)))
        p)))

(defun slot->ir (slot)
  (let* ((name (first slot))
         (rest (rest slot))
         (type (getf rest :type))
         (req  (getf rest :required)))
    `(:obj
      ("name"     . ,(kebab name))
      ("type"     . ,(if (symbolp type) (kebab type) type))
      ("required" . ,(if req t 'false)))))

(defun aggregate->ir (entry)
  (let* ((name  (getf entry :name))
         (src   (getf entry :source-file))
         (rel   (rel-to-contracts-root src))
         (slots (getf entry :slots)))
    `(:obj
      ("kind"   . "aggregate")
      ("name"   . ,(kebab name))
      ("slots"  . (:arr ,@(mapcar #'slot->ir slots)))
      ("source" . (:obj
                   ("dsl_file" . ,rel)
                   ("sha256"   . ,(sha256-of-file src)))))))

;; ---------------------------------------------------------------------------
;; Drive: walk the registry, write one IR file per entry.
;; ---------------------------------------------------------------------------

(defun write-aggregate (entry)
  (let* ((name     (getf entry :name))
         (filename (format nil "ir/domain/~A.ir.json" (kebab name)))
         (data     (aggregate->ir entry)))
    (ensure-directories-exist filename)
    (with-open-file (s filename :direction :output
                                 :if-exists :supersede
                                 :if-does-not-exist :create)
      (emit-json data s 0)
      (write-char #\Newline s))
    (format t "wrote ~A~%" filename)))

(dolist (entry *registry*)
  (case (getf entry :kind)
    (:aggregate (write-aggregate entry))
    (t (error "emit-ir: unknown kind ~S in registry entry ~S"
              (getf entry :kind) entry))))

(sb-ext:exit :code 0)
