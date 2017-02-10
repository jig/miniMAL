(ns miniMAL.step5-tco)

(defn new-env [& [data binds exprs]]
  (atom
    (loop [data (.create js/Object data) b binds e exprs]
      (condp = (first b)
        nil data
        "&" (assoc data (nth b 1) e)
        (recur (assoc data (first b) (first e)) (next b) (next e))))))

(declare EVAL)
(defn eval-ast [ast env]
  (cond (or (array? ast) (seq? ast)) (map #(EVAL % env) ast)
        (and (string? ast) (contains? @env ast)) (get @env ast)
        (string? ast) (throw (str ast " not found"))
        :else ast))

(defn EVAL [ast env]
  (loop [ast ast env env]
    (if (not (array? ast))
      (eval-ast ast env)
      (let [[a0 a1 a2 a3] ast]
        (condp = a0
          "def" (let [e (EVAL a2 env)] (swap! env assoc a1 e) e)
          "let" (let [env (new-env @env)]
                  (doseq [[b e] (partition 2 a1)]
                    (swap! env assoc b (EVAL e env)))
                  (recur a2 env))
          "do" (do (eval-ast (->> ast drop-last rest) env)
                   (recur (last ast) env))
          "if" (if (contains? #{0 nil false ""} (EVAL a1 env))
                 (recur a3 env)
                 (recur a2 env))
          "fn" (with-meta (fn [& a] (EVAL a2 (new-env @env a1 a)))
                          {:ast a2 :env env :params a1})
          (let [[f & el] (eval-ast ast env)
                {:keys [ast env params]} (meta f)]
            (if ast
              (recur ast (new-env @env params el))
              (apply f el))))))))

(def E (new-env {"=" = "<" < "+" + "-" - "*" * "/" /
                 "list" array "map" map}))

(defn -main [& args]
  (let [efn #(%4 nil (js/JSON.stringify (EVAL (js/JSON.parse %1) E)))]
    (.start
      (js/require "repl")
      (clj->js {:eval efn :writer identity :terminal false}))))
