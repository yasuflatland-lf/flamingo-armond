"use client";

import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { useMutation } from "@apollo/client/react";
import { zodResolver } from "@hookform/resolvers/zod";
import { useRouter } from "next/navigation";
import { useForm } from "react-hook-form";
import { Button } from "@/components/ui/button";
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { graphql } from "@/generated";
import { type UpdateProfileFormValues, updateProfileSchema } from "@/schemas/profile";

const UpdateProfileMutation = graphql(`
  mutation UpdateProfile($input: UpdateProfileInput!) {
    updateProfile(input: $input) {
      user {
        id
        displayName
        bio
        avatarUrl
      }
    }
  }
`);

type Props = { initial: { displayName: string; bio: string } };

export function ProfileForm({ initial }: Props) {
  const router = useRouter();
  const form = useForm<UpdateProfileFormValues>({
    resolver: zodResolver(updateProfileSchema),
    defaultValues: {
      displayName: initial.displayName,
      bio: initial.bio || undefined,
    },
  });
  const [updateProfile, { loading, error }] = useMutation(UpdateProfileMutation, {
    onCompleted: () => router.refresh(),
  });

  return (
    <Form {...form}>
      <form
        onSubmit={form.handleSubmit(async (values) => {
          await updateProfile({
            variables: {
              input: {
                displayName: values.displayName,
                bio: values.bio ?? null,
              },
            },
          });
        })}
        className="space-y-4"
      >
        <FormField
          control={form.control}
          name="displayName"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Display name</FormLabel>
              <FormControl>
                <Input {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name="bio"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Bio</FormLabel>
              <FormControl>
                <Input {...field} value={field.value ?? ""} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        {error ? (
          <p className="text-sm text-destructive">
            {CombinedGraphQLErrors.is(error)
              ? (error.errors[0]?.message ?? error.message)
              : error.message}
          </p>
        ) : null}
        <Button type="submit" disabled={loading}>
          {loading ? "Saving..." : "Save"}
        </Button>
      </form>
    </Form>
  );
}
