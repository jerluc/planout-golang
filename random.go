/*
 * Copyright 2014 URX
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package planout

import (
	"crypto/sha1"
	"fmt"
	"strconv"
)

func hash(in string) uint64 {

	// Compute 20- byte sha1
	var x [20]byte = sha1.Sum([]byte(in))

	// Get the first 15 characters of the hexdigest.
	var y string = fmt.Sprintf("%x", x[0:8])
	y = y[0 : len(y)-1]

	// Convert hex string into uint64
	var z uint64 = 0
	z, _ = strconv.ParseUint(y, 16, 64)

	return z
}

func generateNameToHash(units interface{}, interpreter *Interpreter) string {
	unitstr := generateUnitStr(units)
	var salt string = ""
	full_salt, exists := interpreter.get("full_salt")
	if exists {
		salt = full_salt.(string)
	} else {
		current_salt, _ := interpreter.get("salt")
		if len(interpreter.Salt) > 0 {
			salt = interpreter.Salt + "." + current_salt.(string)
		} else {
			salt = current_salt.(string)
		}
	}
	experimentid := salt
	if unitstr != "" {
		experimentid = experimentid + "." + unitstr
	}
	return experimentid
}

func getHash(args map[string]interface{}, interpreter *Interpreter, appended_units ...string) uint64 {
	units := interpreter.evaluate(args["unit"])

	_, exists := args["salt"]
	if exists {
		parameter_salt := interpreter.evaluate(args["salt"])
		interpreter.Inputs["salt"] = parameter_salt.(string)
	}

	name := generateNameToHash(units, interpreter)

	if len(appended_units) > 0 {
		for i := range appended_units {
			name = name + "." + appended_units[i]
		}
	}

	return hash(name)
}

func getUniform(args map[string]interface{}, interpreter *Interpreter, min, max float64, appended_units ...string) float64 {
	scale, _ := strconv.ParseUint("FFFFFFFFFFFFFFF", 16, 64)
	append_string := ""
	var h uint64 = 0
	if len(appended_units) == 0 {
		h = getHash(args, interpreter)
	} else {
		append_string = append_string + appended_units[0]
		for i := range appended_units {
			if i > 0 {
				append_string = append_string + "." + appended_units[i]
			}
		}
		h = getHash(args, interpreter, append_string)
	}
	shift := float64(h) / float64(scale)
	return min + shift*(max-min)
}

type uniformChoice struct{}

func (s *uniformChoice) execute(args map[string]interface{}, interpreter *Interpreter) interface{} {
	existOrPanic(args, []string{"choices", "unit"}, "UniformChoice")
	choices := interpreter.evaluate(args["choices"]).([]interface{})
	nchoices := uint64(len(choices))
	idx := getHash(args, interpreter) % nchoices
	choice := choices[idx]
	return choice
}

type bernoulliTrial struct{}

func (s *bernoulliTrial) execute(args map[string]interface{}, interpreter *Interpreter) interface{} {
	existOrPanic(args, []string{"unit"}, "BernoulliTrial")
	pvalue := interpreter.evaluate(args["p"]).(float64)
	rand_val := getUniform(args, interpreter, 0.0, 1.0)
	if rand_val <= pvalue {
		return 1
	}
	return 0
}

type bernoulliFilter struct{}

func (s *bernoulliFilter) execute(args map[string]interface{}, interpreter *Interpreter) interface{} {
	existOrPanic(args, []string{"choices", "unit"}, "BernoulliFilter")
	pvalue := interpreter.evaluate(args["p"]).(float64)
	choices := interpreter.evaluate(args["choices"]).([]interface{})
	ret := make([]interface{}, 0, len(choices))
	for i := range choices {
		append_str, _ := toString(choices[i])
		rand_val := getUniform(args, interpreter, 0.0, 1.0, append_str)
		if rand_val <= pvalue {
			ret = append(ret, choices[i])
		}
	}
	return ret
}

type weightedChoice struct{}

func (s *weightedChoice) execute(args map[string]interface{}, interpreter *Interpreter) interface{} {
	existOrPanic(args, []string{"choices", "unit", "weights"}, "WeightedChoice")
	weights := interpreter.evaluate(args["weights"]).([]interface{})
	sum, cweights := getCummulativeWeights(weights)
	stop_val := getUniform(args, interpreter, 0.0, sum)
	choices := interpreter.evaluate(args["choices"]).([]interface{})
	for i := range cweights {
		if stop_val <= cweights[i] {
			return choices[i]
		}
	}
	return 0.0
}

type randomFloat struct{}

func (s *randomFloat) execute(args map[string]interface{}, interpreter *Interpreter) interface{} {
	existOrPanic(args, []string{"unit"}, "RandomFloat")
	min_val, _ := toNumber(getOrElse(args, "min", 0.0))
	max_val, _ := toNumber(getOrElse(args, "max", 1.0))
	return getUniform(args, interpreter, min_val, max_val)
}

type randomInteger struct{}

func (s *randomInteger) execute(args map[string]interface{}, interpreter *Interpreter) interface{} {
	existOrPanic(args, []string{"unit"}, "RandomFloat")
	min_val, _ := toNumber(getOrElse(args, "min", 0.0))
	max_val, _ := toNumber(getOrElse(args, "max", 0.0))
	mod_val := uint64(max_val) - uint64(min_val) + 1
	return uint64(min_val) + getHash(args, interpreter)%mod_val
}

type sample struct{}

func (s *sample) execute(args map[string]interface{}, interpreter *Interpreter) interface{} {
	existOrPanic(args, []string{"unit", "choices"}, "Sample")
	choices := interpreter.evaluate(args["choices"]).([]interface{})
	nhash := getHash(args, interpreter)
	FisherYatesShuffle(choices, nhash)
	draws, _ := toNumber(getOrElse(args, "draws", len(choices)))
	return choices[:int(draws)]
}
