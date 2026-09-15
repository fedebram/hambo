package cli

func NoArgs(args []string) error {
	if len(args) != 0 {
		return UsageErrorf("unexpected argument %q", args[0])
	}
	return nil
}

func ExactArgs(count int) ArgsValidator {
	return func(args []string) error {
		if len(args) < count {
			return UsageErrorf("missing required argument")
		}
		if len(args) > count {
			return UsageErrorf("unexpected argument %q", args[count])
		}
		return nil
	}
}
