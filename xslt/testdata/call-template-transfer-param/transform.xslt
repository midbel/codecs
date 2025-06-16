<?xml version="1.0" encoding="UTF-8"?>

<xsl:stylesheet version="3.0" 
	xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
	<xsl:output method="xml" indent="yes"/>
	<xsl:template match="/">
		<item>
			<value>
				<xsl:value-of select="/root/item"/>
			</value>
			<xsl:call-template name="foobar">
				<xsl:with-param name="b" select="/root/item"/>
			</xsl:call-template>
			<xsl:call-template name="build-with">
				<xsl:with-param name="build" select="/root/item"/>
			</xsl:call-template>
		</item>
	</xsl:template>

	<xsl:template name="build-with">
		<xsl:param name="build"/>
		<info>
			<xsl:call-template name="foobar">
				<xsl:with-param name="b" select="$build"/>
			</xsl:call-template>
		</info>
	</xsl:template>

	<xsl:template name="foobar">
		<xsl:param name="b" select="'angle'"/>
		<build-with>
			<xsl:value-of select="$b"/>
		</build-with>
	</xsl:template>

</xsl:stylesheet>