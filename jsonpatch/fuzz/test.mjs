import { spawn } from "node:child_process";
import crypto from "node:crypto";
import { diffLines } from "diff";
import { enablePatches, produceWithPatches } from "immer";
import {
	OpAdd,
	OpInc,
	OpRemove,
	OpReplace,
	OpStrDel,
	OpStrIns,
} from "json-joy/lib/json-patch/op/index.js";
import { diff } from "json-joy/lib/util/diff/str.js";

const INS = 1;
const DEL = -1;


enablePatches();

function unicodeLength(str) {
        return Array.from(str).length;
}

function unicodeInsert(str, index, insertStr) {
        const arr = Array.from(str);
        arr.splice(index, 0, ...Array.from(insertStr));
        return arr.join("");
}

function unicodeDelete(str, start, deleteLen) {
        const arr = Array.from(str);
        arr.splice(start, deleteLen);
        return arr.join("");
}

function unicodeReplace(str, index, replaceStr) {
        const arr = Array.from(str);
        arr.splice(index, 1, ...Array.from(replaceStr));
        return arr.join("");
}

// Unicode test strings with various complexities
const UNICODE_STRINGS = [
	"Hello World",
	"Hello 🌍 World",
	"こんにちは世界", // Japanese
	"Héllo Wörld", // Accented characters
	"𝕳𝖊𝖑𝖑𝖔 𝖂𝖔𝖗𝖑𝖉", // Mathematical bold
	"🚀🌟💫⭐🌙", // Emoji sequence
	"café naïve résumé", // French accents
	"Привет мир", // Cyrillic
	"مرحبا بالعالم", // Arabic
	"👨‍💻👩‍🔬🧑‍🎨", // Complex emoji with ZWJ sequences
];

function generateRandomString() {
	const baseString =
		UNICODE_STRINGS[Math.floor(Math.random() * UNICODE_STRINGS.length)];
	const extraChars = Math.random() < 0.5 ? "" : " extra text";
	return baseString + extraChars;
}

function generateRandomValue(depth = 0) {
	const maxDepth = 4;
	if (depth >= maxDepth) {
		// At max depth, only return primitives
		const primitives = [
			Math.floor(Math.random() * 1000),
			Math.random() * 100,
			generateRandomString(),
			Math.random() < 0.5,
			null,
		];
		return primitives[Math.floor(Math.random() * primitives.length)];
	}

	const types = ["number", "string", "boolean", "null", "array", "object"];
	const type = types[Math.floor(Math.random() * types.length)];

	switch (type) {
		case "number":
			return Math.random() < 0.5
				? Math.floor(Math.random() * 1000)
				: Math.random() * 100;
		case "string":
			return generateRandomString();
		case "boolean":
			return Math.random() < 0.5;
		case "null":
			return null;
		case "array": {
			const length = Math.floor(Math.random() * 5) + 1;
			return Array.from({ length }, () => generateRandomValue(depth + 1));
		}
		case "object": {
			const keys = Math.floor(Math.random() * 5) + 1;
			const obj = {};
			for (let i = 0; i < keys; i++) {
				const key = `key${i}_${Math.random().toString(36).substr(2, 5)}`;
				obj[key] = generateRandomValue(depth + 1);
			}
			return obj;
		}
	}
}

function generateRandomDocument() {
	const baseDoc = {
		// Ensure we have some predictable paths for testing
		counter: Math.floor(Math.random() * 100),
		text: generateRandomString(),
		flag: Math.random() < 0.5,
		nested: {
			value: generateRandomString(),
			count: Math.floor(Math.random() * 50),
			array: Array.from({ length: 3 }, () => generateRandomValue(1)),
		},
		list: Array.from({ length: 4 }, () => generateRandomValue(1)),
	};

	// Add some random additional fields
	const extraFields = Math.floor(Math.random() * 5) + 2;
	for (let i = 0; i < extraFields; i++) {
		const key = `random${i}_${Math.random().toString(36).substr(2, 5)}`;
		baseDoc[key] = generateRandomValue(0);
	}

	return baseDoc;
}

function applyRandomMutations(draft) {
	const mutations = Math.floor(Math.random() * 10) + 2; // 2-11 mutations

	for (let i = 0; i < mutations; i++) {
		try {
			const mutationType = Math.floor(Math.random() * 10);

			switch (mutationType) {
				case 0: // Increment counter
					if (typeof draft.counter === "number") {
						draft.counter += Math.floor(Math.random() * 10) - 5;
					}
					break;
				case 1: // Inline text editing - comprehensive mutations
                                        if (typeof draft.text === "string" && unicodeLength(draft.text) > 0) {
						const operations = [
							"replace",
							"prepend",
							"append",
							"insert",
							"delete",
							"substitute",
							"split_insert",
							"unicode_insert",
						];
						const op =
							operations[Math.floor(Math.random() * operations.length)];
						switch (op) {
							case "replace":
								draft.text = generateRandomString();
								break;
							case "prepend":
								draft.text = `PREFIX: ${draft.text}`;
								break;
							case "append":
								draft.text = `${draft.text} :SUFFIX`;
								break;
                                                        case "insert": {
                                                                const pos = Math.floor(Math.random() * (unicodeLength(draft.text) + 1));
                                                                draft.text = unicodeInsert(draft.text, pos, "[INS]");
                                                                break;
                                                        }
                                                        case "delete": {
                                                                if (unicodeLength(draft.text) > 1) {
                                                                        const start = Math.floor(Math.random() * unicodeLength(draft.text));
                                                                        const end = Math.min(
                                                                               start + Math.floor(Math.random() * 3) + 1,
                                                                               unicodeLength(draft.text),
                                                                        );
                                                                        draft.text = unicodeDelete(draft.text, start, end - start);
                                                                }
                                                                break;
                                                        }
                                                        case "substitute": {
                                                                const pos = Math.floor(Math.random() * unicodeLength(draft.text));
                                                                const replacement = ["X", "@", "#", "*"][
                                                                        Math.floor(Math.random() * 4)
                                                                ];
                                                                draft.text = unicodeReplace(draft.text, pos, replacement);
                                                                break;
                                                        }
                                                        case "split_insert": {
                                                                const pos = Math.floor(Math.random() * (unicodeLength(draft.text) + 1));
                                                                const insertText = ` | ${generateRandomString().slice(0, 10)} | `;
                                                                draft.text = unicodeInsert(draft.text, pos, insertText);
                                                                break;
                                                        }
                                                        case "unicode_insert": {
                                                                const pos = Math.floor(Math.random() * (unicodeLength(draft.text) + 1));
                                                                const unicodeChars = ["🔧", "⚡", "🎯", "✨", "🚀", "💡", "🔥"];
                                                                const char =
                                                                        unicodeChars[Math.floor(Math.random() * unicodeChars.length)];
                                                                draft.text = unicodeInsert(draft.text, pos, char);
                                                                break;
                                                        }
						}
					}
					break;
				case 2: // Toggle flag
					draft.flag = !draft.flag;
					break;
				case 3: // Modify nested value
					if (draft.nested && typeof draft.nested.value === "string") {
						draft.nested.value = generateRandomString();
					}
					break;
				case 4: // Increment nested count
					if (draft.nested && typeof draft.nested.count === "number") {
						draft.nested.count += Math.floor(Math.random() * 10) - 5;
					}
					break;
				case 5: // Add to array
					if (Array.isArray(draft.list)) {
						draft.list.push(generateRandomValue(2));
					}
					break;
				case 6: // Remove from array
					if (Array.isArray(draft.list) && draft.list.length > 1) {
						const index = Math.floor(Math.random() * draft.list.length);
						draft.list.splice(index, 1);
					}
					break;
				case 7: // Modify nested array
					if (
						draft.nested &&
						Array.isArray(draft.nested.array) &&
						draft.nested.array.length > 0
					) {
						const index = Math.floor(Math.random() * draft.nested.array.length);
						draft.nested.array[index] = generateRandomValue(2);
					}
					break;
                                case 8: // Inline string deletion
                                        if (typeof draft.text === "string" && unicodeLength(draft.text) > 3) {
                                                const deleteLen = Math.floor(Math.random() * 5) + 1;
                                                const start = Math.floor(
                                                        Math.random() * (unicodeLength(draft.text) - deleteLen),
                                                );
                                                draft.text = unicodeDelete(draft.text, start, deleteLen);
                                        }
					break;
                                case 9: // Complex string operations
                                        if (typeof draft.text === "string" && unicodeLength(draft.text) > 2) {
						// Random combination of insert, delete, and replace
						const operations = Math.floor(Math.random() * 3) + 1;
                                                for (let op = 0; op < operations; op++) {
                                                        const opType = Math.floor(Math.random() * 3);
                                                        if (unicodeLength(draft.text) === 0) break;
                                                        switch (opType) {
                                                                case 0: // insert
                                                                        if (unicodeLength(draft.text) < 100) {
                                                                               const pos = Math.floor(
                                                                               Math.random() * (unicodeLength(draft.text) + 1),
                                                                               );
                                                                               draft.text = unicodeInsert(draft.text, pos, "*");
                                                                        }
                                                                        break;
                                                                case 1: // delete
                                                                        if (unicodeLength(draft.text) > 1) {
                                                                               const pos = Math.floor(Math.random() * unicodeLength(draft.text));
                                                                               draft.text = unicodeDelete(draft.text, pos, 1);
                                                                        }
                                                                        break;
                                                                case 2: // replace
                                                                        if (unicodeLength(draft.text) > 0) {
                                                                               const pos = Math.floor(Math.random() * unicodeLength(draft.text));
                                                                               draft.text = unicodeReplace(draft.text, pos, "~");
                                                                        }
                                                                        break;
                                                        }
						}
					}
					break;
			}
		} catch (e) {
			// Ignore mutation errors and continue
			console.warn(`Mutation ${i} failed:`, e.message);
		}
	}
}

async function runSingleFuzzTest(testId) {
	try {
		const originalDoc = generateRandomDocument();

		const [newDoc, patches, inversePatches] = produceWithPatches(
			originalDoc,
			applyRandomMutations,
		);

		const ops = convertImmerPatchesToJsonJoyOps(patches, inversePatches);
		const operations = ops.map((op) => op.toJson());

		const testCase = {
			testId,
			originalDoc,
			expectedDoc: newDoc,
			operations,
		};

		return await testWithGo(testCase);
	} catch (error) {
		return {
			testId,
			success: false,
			error: `JS error:\n${error instanceof Error ? error.stack : String(error)}`,
			jsError: true,
		};
	}
}

async function testWithGo(testCase) {
	return new Promise((resolve) => {
		const harnessPath =
			process.env.TEST_HARNESS_PATH || "../../cmd/test-harness/main.go";
		const isBuiltBinary = !harnessPath.endsWith(".go");

		const goProcess = isBuiltBinary
			? spawn(harnessPath, [], {
					cwd: process.cwd(),
					stdio: ["pipe", "pipe", "pipe"],
				})
			: spawn("go", ["run", harnessPath], {
					cwd: process.cwd(),
					stdio: ["pipe", "pipe", "pipe"],
				});

		let stdoutData = "";
		let stderrData = "";

		goProcess.stdout.on("data", (data) => {
			stdoutData += data.toString();
		});

		goProcess.stderr.on("data", (data) => {
			stderrData += data.toString();
		});

		goProcess.on("close", (code) => {
			if (code !== 0) {
				resolve({
					testId: testCase.testId,
					success: false,
					error: `Go process exited with code ${code}: ${stderrData}`,
					goError: true,
				});
				return;
			}

			try {
				const result = JSON.parse(stdoutData.trim());

				if (result.success) {
					// Compare documents
					const documentsMatch = deepEqual(
						result.resultDoc,
						testCase.expectedDoc,
					);
					if (!documentsMatch) {
						resolve({
							testId: testCase.testId,
							success: false,
							error: "Document mismatch",
							expectedDoc: testCase.expectedDoc,
							actualDoc: result.resultDoc,
							operations: testCase.operations,
						});
					} else {
						resolve({
							testId: testCase.testId,
							success: true,
						});
					}
				} else {
					resolve({
						testId: testCase.testId,
						success: false,
						error: result.error,
						goError: true,
					});
				}
			} catch (parseError) {
				resolve({
					testId: testCase.testId,
					success: false,
					error: `Failed to parse Go response: ${parseError.message}`,
					stdout: stdoutData,
					stderr: stderrData,
				});
			}
		});

		goProcess.stdin.write(`${JSON.stringify(testCase)}\n`);
		goProcess.stdin.end();
	});
}

function deepEqual(a, b) {
	if (a === b) return true;
	if (a == null || b == null) return false;
	if (typeof a !== typeof b) return false;

	if (typeof a === "object") {
		if (Array.isArray(a) !== Array.isArray(b)) return false;

		if (Array.isArray(a)) {
			if (a.length !== b.length) return false;
			for (let i = 0; i < a.length; i++) {
				if (!deepEqual(a[i], b[i])) return false;
			}
			return true;
		}
		const keysA = Object.keys(a);
		const keysB = Object.keys(b);
		if (keysA.length !== keysB.length) return false;

		for (const key of keysA) {
			if (!keysB.includes(key)) return false;
			if (!deepEqual(a[key], b[key])) return false;
		}
		return true;
	}

	return false;
}

function formatJsonDiff(expected, actual) {
	// Recursively sort all object keys for consistent ordering
	function sortKeys(obj) {
		if (Array.isArray(obj)) {
			return obj.map(sortKeys);
		}
		if (obj !== null && typeof obj === "object") {
			const sorted = {};
			for (const key of Object.keys(obj).sort()) {
				sorted[key] = sortKeys(obj[key]);
			}
			return sorted;
		}
		return obj;
	}

	const expectedSorted = sortKeys(expected);
	const actualSorted = sortKeys(actual);

	const expectedJson = JSON.stringify(expectedSorted, null, 2);
	const actualJson = JSON.stringify(actualSorted, null, 2);

	const diff = diffLines(expectedJson, actualJson);

	const result = [];
	let hasChanges = false;

	for (let i = 0; i < diff.length; i++) {
		const part = diff[i];
		if (part.added || part.removed) {
			hasChanges = true;
		}
	}

	if (!hasChanges) {
		return "No differences found (this shouldn't happen!)";
	}

	// Build clean diff with proper context
	result.push("\x1b[31m--- Expected  (JavaScript result)\x1b[0m");
	result.push("\x1b[32m+++ Actual    (Go result)\x1b[0m");

	for (let i = 0; i < diff.length; i++) {
		const part = diff[i];
		const partLines = part.value.split("\n").filter((line) => line !== "");

		if (part.added) {
			for (const line of partLines) {
				result.push(`\x1b[31m+ ${line}\x1b[0m`);
			}
		} else if (part.removed) {
			for (const line of partLines) {
				result.push(`\x1b[32m- ${line}\x1b[0m`);
			}
		} else {
			// Context lines - only show if adjacent to changes
			const hasAdjacentChanges =
				(i > 0 && (diff[i - 1].added || diff[i - 1].removed)) ||
				(i < diff.length - 1 && (diff[i + 1].added || diff[i + 1].removed));

			if (hasAdjacentChanges) {
				// Show limited context around changes
				const contextLines = partLines;
				const prevHasChanges =
					i > 0 && (diff[i - 1].added || diff[i - 1].removed);
				const nextHasChanges =
					i < diff.length - 1 && (diff[i + 1].added || diff[i + 1].removed);

				let startIdx = 0;
				let endIdx = contextLines.length;

				if (prevHasChanges && !nextHasChanges) {
					// Show first 3 lines after previous change
					endIdx = Math.min(3, contextLines.length);
				} else if (!prevHasChanges && nextHasChanges) {
					// Show last 3 lines before next change
					startIdx = Math.max(0, contextLines.length - 3);
				} else if (prevHasChanges && nextHasChanges) {
					// Show middle context, up to 6 lines total
					if (contextLines.length > 6) {
						startIdx = Math.max(0, Math.floor(contextLines.length / 2) - 3);
						endIdx = startIdx + 6;
					}
				}

				for (let j = startIdx; j < endIdx; j++) {
					result.push(`  ${contextLines[j]}`);
				}
			}
		}
	}

	return result.join("\n");
}

function formatOperationsSummary(operations) {
	const summary = operations
		.map((op, index) => {
			const { op: opType, path, ...rest } = op;
			const restStr = Object.entries(rest)
				.map(([key, value]) => {
					if (typeof value === "string" && value.length > 50) {
						return `${key}: "${value.substring(0, 50)}..."`;
					}
					return `${key}: ${JSON.stringify(value)}`;
				})
				.join(", ");
			return `  ${index + 1}. ${opType} ${path}${restStr ? ` (${restStr})` : ""}`;
		})
		.join("\n");

	return summary;
}

function findStringDifferences(expected, actual, path = "") {
	const differences = [];

	function traverse(exp, act, currentPath) {
		if (typeof exp === "string" && typeof act === "string" && exp !== act) {
			differences.push({
				path: currentPath,
				expected: exp,
				actual: act,
				issue: analyzeStringDifference(exp, act),
			});
		} else if (
			typeof exp === "object" &&
			typeof act === "object" &&
			exp !== null &&
			act !== null
		) {
			if (Array.isArray(exp) && Array.isArray(act)) {
				const maxLen = Math.max(exp.length, act.length);
				for (let i = 0; i < maxLen; i++) {
					if (i < exp.length && i < act.length) {
						traverse(exp[i], act[i], `${currentPath}[${i}]`);
					}
				}
			} else if (!Array.isArray(exp) && !Array.isArray(act)) {
				const allKeys = new Set([...Object.keys(exp), ...Object.keys(act)]);
				for (const key of allKeys) {
					if (key in exp && key in act) {
						traverse(
							exp[key],
							act[key],
							currentPath ? `${currentPath}.${key}` : key,
						);
					}
				}
			}
		}
	}

	traverse(expected, actual, path);
	return differences;
}

function analyzeStringDifference(expected, actual) {
	// Check for Unicode corruption indicators
	const hasReplacementChar = actual.includes("�");
	const hasUnicodeEmoji =
		/[\u{1F600}-\u{1F64F}\u{1F300}-\u{1F5FF}\u{1F680}-\u{1F6FF}\u{1F1E0}-\u{1F1FF}\u{2600}-\u{26FF}\u{2700}-\u{27BF}]/u.test(
			expected,
		);
	const hasMathSymbols = /[\u{1D400}-\u{1D7FF}]/u.test(expected);

	if (hasReplacementChar) {
		return "🚨 Unicode corruption detected (� replacement characters)";
	}
	if (hasUnicodeEmoji && expected.length !== actual.length) {
		return "🔀 Possible emoji/surrogate pair offset issue";
	}
	if (hasMathSymbols && expected !== actual) {
		return "🔢 Mathematical symbol handling difference";
	}
	if (expected.length !== actual.length) {
		return `📏 Length mismatch (expected: ${expected.length}, actual: ${actual.length})`;
	}
	return "❓ String content differs";
}

async function runFuzzTests(numTests = 100) {
	console.log(`🚀 Starting fuzz testing with ${numTests} test cases...`);

	let passed = 0;
	let failed = 0;
	const failures = [];

	for (let i = 0; i < numTests; i++) {
		const testId = crypto.randomUUID();
		const result = await runSingleFuzzTest(testId);

		if (result.success) {
			passed++;
			process.stdout.write(".");
		} else {
			failed++;
			failures.push(result);
			process.stdout.write("F");
		}

		if ((i + 1) % 50 === 0) {
			console.log(
				`\n[${i + 1}/${numTests}] Passed: ${passed}, Failed: ${failed}`,
			);
		}
	}

	console.log("\n\n📊 Fuzz Test Results:");
	console.log(`✅ Passed: ${passed}`);
	console.log(`❌ Failed: ${failed}`);
	console.log(`📈 Success Rate: ${((passed / numTests) * 100).toFixed(2)}%`);

	if (failures.length > 0) {
		console.log("\n🔍 First 5 Failures:");
		failures.slice(0, 5).forEach((failure, index) => {
			console.log(`\n${"=".repeat(80)}`);
			console.log(`❌ Failure ${index + 1}: ${failure.testId}`);
			console.log(`${"=".repeat(80)}`);
			console.log(`🚨 Error: ${failure.error}`);

			if (failure.operations && failure.operations.length > 0) {
				console.log(
					`\n📝 Operations Applied (${failure.operations.length} total):`,
				);
				console.log(formatOperationsSummary(failure.operations));
			}

			if (failure.expectedDoc && failure.actualDoc) {
				// Find and highlight string differences first
				const stringDiffs = findStringDifferences(
					failure.expectedDoc,
					failure.actualDoc,
				);
				if (stringDiffs.length > 0) {
					console.log("\n🔍 String Differences Found:");
					for (const diff of stringDiffs) {
						console.log(`  📍 ${diff.path}: ${diff.issue}`);
						console.log(`    Expected: "${diff.expected}"`);
						console.log(`    Actual:   "${diff.actual}"`);
					}
				}

				console.log("\n📊 Full Document Comparison:");
				console.log(formatJsonDiff(failure.expectedDoc, failure.actualDoc));
			}

			if (failure.stdout || failure.stderr) {
				console.log("\n🔧 Go Process Output:");
				if (failure.stdout) console.log("STDOUT:", failure.stdout);
				if (failure.stderr) console.log("STDERR:", failure.stderr);
			}
		});

		if (failures.length > 5) {
			console.log(`\n... and ${failures.length - 5} more failures not shown`);
			console.log("💡 Tip: Run with fewer test cases to see all failures");
		}

		// Categorize failures
		const categories = {
			documentMismatch: failures.filter((f) => f.error === "Document mismatch")
				.length,
			goErrors: failures.filter((f) => f.goError).length,
			jsErrors: failures.filter((f) => f.jsError).length,
			other: failures.filter(
				(f) => !f.goError && !f.jsError && f.error !== "Document mismatch",
			).length,
		};

		console.log("\n📋 Failure Summary:");
		if (categories.documentMismatch > 0) {
			console.log(
				`  🔀 Document Mismatches: ${categories.documentMismatch} (JS-Go result differences)`,
			);
		}
		if (categories.goErrors > 0) {
			console.log(
				`  🔴 Go Process Errors: ${categories.goErrors} (errors in Go harness)`,
			);
		}
		if (categories.jsErrors > 0) {
			console.log(
				`  🔶 JavaScript Errors: ${categories.jsErrors} (errors in JS generation)`,
			);
		}
		if (categories.other > 0) {
			console.log(`  ❓ Other Errors: ${categories.other}`);
		}
	}

	return { passed, failed, failures };
}

// Run the main fuzz test
const numTests = process.argv[2] ? Number.parseInt(process.argv[2]) : 100;
runFuzzTests(numTests).catch(console.error);

function convertImmerPatchesToJsonJoyOps(immerPatches, inverseImmerPatches) {
	const ops = [];
	for (let i = 0; i < immerPatches.length; i++) {
		const forwardPatch = immerPatches[i];
		const inversePatch = inverseImmerPatches[i];
		if (!forwardPatch || !inversePatch) {
			throw new Error("internal error: forward or inverse patch is null");
		}

		switch (forwardPatch.op) {
			case "add":
				ops.push(new OpAdd(forwardPatch.path, forwardPatch.value));
				break;
			case "remove":
				ops.push(new OpRemove(forwardPatch.path, undefined));
				break;
			case "replace": {
				const oldValue = inversePatch.value;
				const newValue = forwardPatch.value;

				if (typeof newValue === "number" && typeof oldValue === "number") {
					const diff = newValue - oldValue;
					if (diff === 1 || diff === -1) {
						ops.push(new OpInc(forwardPatch.path, diff));
					} else {
						ops.push(new OpReplace(forwardPatch.path, newValue, undefined));
					}
				} else if (
					typeof newValue === "string" &&
					typeof oldValue === "string"
				) {
					const stringDiffs = diff(oldValue, newValue);
					let adjustedIndex = 0;
					for (const [type, text] of stringDiffs) {
						if (type === INS) {
							ops.push(new OpStrIns(forwardPatch.path, adjustedIndex, text));
							adjustedIndex += text.length;
						} else if (type === DEL) {
							ops.push(
								new OpStrDel(forwardPatch.path, adjustedIndex, text, undefined),
							);
						} else {
							adjustedIndex += text.length;
						}
					}
				} else {
					ops.push(new OpReplace(forwardPatch.path, newValue, undefined));
				}
				break;
			}
			default:
				throw new Error(
					`internal error: unknown operation: ${forwardPatch.op}`,
				);
		}
	}
	return ops;
}
