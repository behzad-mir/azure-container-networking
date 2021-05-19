package npm

import (
	"container/heap"
	"fmt"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Azure/azure-container-networking/log"
	"github.com/Azure/azure-container-networking/npm/util"
)

// An ReqHeap is a min-heap of labelSelectorRequirements.
type ReqHeap []metav1.LabelSelectorRequirement

func (h ReqHeap) Len() int {
	return len(h)
}

func (h ReqHeap) Less(i, j int) bool {
	sort.Strings(h[i].Values)
	sort.Strings(h[j].Values)

	if int(h[i].Key[0]) < int(h[j].Key[0]) {
		return true
	}

	if int(h[i].Key[0]) > int(h[j].Key[0]) {
		return false
	}

	if len(h[i].Values) == 0 {
		return true
	}

	if len(h[j].Values) == 0 {
		return false
	}

	if len(h[i].Values[0]) == 0 {
		return true
	}

	if len(h[j].Values[0]) == 0 {
		return false
	}

	return int(h[i].Values[0][0]) < int(h[j].Values[0][0])
}

func (h ReqHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *ReqHeap) Push(x interface{}) {
	sort.Strings(x.(metav1.LabelSelectorRequirement).Values)
	*h = append(*h, x.(metav1.LabelSelectorRequirement))
}

func (h *ReqHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]

	return x
}

// ParseLabel takes a Azure-NPM processed label then returns if it's referring to complement set,
// and if so, returns the original set as well.
func ParseLabel(label string) (string, bool) {
	//The input label is guaranteed to have a non-zero length validated by k8s.
	//For label definition, see below parseSelector() function.
	if label[0:1] == util.IptablesNotFlag {
		return label[1:], true
	}
	return label, false
}

// GetOperatorAndLabel returns the operator associated with the label and the label without operator.
func GetOperatorAndLabel(label string) (string, string) {
	if len(label) == 0 {
		return "", ""
	}

	if string(label[0]) == util.IptablesNotFlag {
		return util.IptablesNotFlag, label[1:]
	}

	return "", label
}

// GetOperatorsAndLabels returns the operators along with the associated labels.
func GetOperatorsAndLabels(labelsWithOps []string) ([]string, []string) {
	var ops, labelsWithoutOps []string
	for _, labelWithOp := range labelsWithOps {
		op, labelWithoutOp := GetOperatorAndLabel(labelWithOp)
		ops = append(ops, op)
		labelsWithoutOps = append(labelsWithoutOps, labelWithoutOp)
	}

	return ops, labelsWithoutOps
}

// sortSelector sorts the member fields of the selector in an alphebatical order.
func sortSelector(selector *metav1.LabelSelector) {
	_, _ = util.SortMap(&selector.MatchLabels)

	reqHeap := &ReqHeap{}
	heap.Init(reqHeap)
	for _, req := range selector.MatchExpressions {
		heap.Push(reqHeap, req)
	}

	var sortedReqs []metav1.LabelSelectorRequirement
	for reqHeap.Len() > 0 {
		sortedReqs = append(sortedReqs, heap.Pop(reqHeap).(metav1.LabelSelectorRequirement))

	}
	selector.MatchExpressions = sortedReqs
}

// getSetNameForMultiValueSelector takes in label with multiple values without operator
// and returns a new 2nd level ipset name
func getSetNameForMultiValueSelector(key string, vals []string) string {
	returnString := key
	for _, val := range vals {
		returnString = util.GetIpSetFromLabelKV(returnString, val)
	}
	return returnString
}

// HashSelector returns the hash value of the selector.
func HashSelector(selector *metav1.LabelSelector) string {
	sortSelector(selector)
	return util.Hash(fmt.Sprintf("%v", selector))
}

// flattenNameSpaceSelector will help flatten multiple NameSpace selector match Expressions values
// into multiple label selectors helping with the OR condition.
func FlattenNameSpaceSelector(nsSelector *metav1.LabelSelector) []metav1.LabelSelector {
	//egress section
	// for _, egressRule := range nsSelector {
	// 	log.Logf("%+v", egressRule)
	// }
	if nsSelector == nil {
		return []metav1.LabelSelector{*nsSelector}
	}

	if len(nsSelector.MatchExpressions) == 0 {
		return []metav1.LabelSelector{*nsSelector}
	}

	baseSelector := &metav1.LabelSelector{
		MatchLabels:      nsSelector.MatchLabels,
		MatchExpressions: []metav1.LabelSelectorRequirement{},
	}

	multiValuePresent := false
	multiValueMatchExprs := []metav1.LabelSelectorRequirement{}
	for _, req := range nsSelector.MatchExpressions {

		if (req.Operator == metav1.LabelSelectorOpIn) || (req.Operator == metav1.LabelSelectorOpNotIn) {
			if len(req.Values) == 1 {
				baseSelector.MatchExpressions = append(baseSelector.MatchExpressions, req)
				continue
			}
			multiValuePresent = true
			multiValueMatchExprs = append(multiValueMatchExprs, req)

		} else if (req.Operator == metav1.LabelSelectorOpExists) || (req.Operator == metav1.LabelSelectorOpDoesNotExist) {
			baseSelector.MatchExpressions = append(baseSelector.MatchExpressions, req)
		} else {
			log.Errorf("Invalid operator [%s] for selector [%v] requirement", req.Operator, *nsSelector)
		}
	}

	// If there are no multiValue NS selector match expressions
	// return the original NsSelector
	if !multiValuePresent {
		return []metav1.LabelSelector{*nsSelector}
	}

	// Now use the baseSelector and loop over multiValueMatchExprs to create all
	// combinations of values
	flatNsSelectors := []metav1.LabelSelector{
		*baseSelector.DeepCopy(),
	}
	for _, req := range multiValueMatchExprs {
		flatNsSelectors = zipMatchExprs(flatNsSelectors, req)
	}

	return flatNsSelectors
}

func zipMatchExprs(baseSelectors []metav1.LabelSelector, matchExpr metav1.LabelSelectorRequirement) []metav1.LabelSelector {
	zippedLabelSelectors := []metav1.LabelSelector{}
	for _, selector := range baseSelectors {
		for _, value := range matchExpr.Values {
			tempBaseSelector := selector.DeepCopy()
			tempBaseSelector.MatchExpressions = append(
				tempBaseSelector.MatchExpressions,
				metav1.LabelSelectorRequirement{
					Key:      matchExpr.Key,
					Operator: matchExpr.Operator,
					Values:   []string{value},
				},
			)
			zippedLabelSelectors = append(zippedLabelSelectors, *tempBaseSelector)

		}
	}
	return zippedLabelSelectors
}

// parseSelector takes a LabelSelector and returns a slice of processed labels, Lists with members as values.
// this function returns a slice of all the label ipsets excluding multivalue matchExprs
// and a map of labelKeys and labelIpsetname for multivalue match exprs
// higher level functions will need to compute what sets or ipsets should be
// used from this map
func parseSelector(selector *metav1.LabelSelector) ([]string, map[string][]string) {
	var (
		labels []string
		vals   map[string][]string
	)

	vals = make(map[string][]string)
	if selector == nil {
		return labels, vals
	}

	if len(selector.MatchLabels) == 0 && len(selector.MatchExpressions) == 0 {
		labels = append(labels, "")
		return labels, vals
	}

	sortedKeys, sortedVals := util.SortMap(&selector.MatchLabels)

	for i := range sortedKeys {
		labels = append(labels, sortedKeys[i]+":"+sortedVals[i])
	}

	for _, req := range selector.MatchExpressions {
		var k string
		switch op := req.Operator; op {
		case metav1.LabelSelectorOpIn:
			k = req.Key
			if len(req.Values) == 1 {
				labels = append(labels, k+":"+req.Values[0])
				continue
			}
			// We are not adding the k:v to labels for multiple values, because, labels are used
			// to contruct partial IptEntries and if these below labels are added then we are inducing
			// AND condition on values of a match expression instead of OR
			for _, v := range req.Values {
				vals[k] = append(vals[k], v)
			}
		case metav1.LabelSelectorOpNotIn:
			k = util.IptablesNotFlag + req.Key
			if len(req.Values) == 1 {
				labels = append(labels, k+":"+req.Values[0])
				continue
			}
			for _, v := range req.Values {
				vals[k] = append(vals[k], v)
			}
		// Exists matches pods with req.Key as key
		case metav1.LabelSelectorOpExists:
			k = req.Key
			labels = append(labels, k)
		// DoesNotExist matches pods without req.Key as key
		case metav1.LabelSelectorOpDoesNotExist:
			k = util.IptablesNotFlag + req.Key
			labels = append(labels, k)
		default:
			log.Errorf("Invalid operator [%s] for selector [%v] requirement", op, *selector)
		}
	}

	return labels, vals
}
